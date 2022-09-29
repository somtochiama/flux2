/*
Copyright 2021 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package test

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"io"
	"log"
	"os/exec"

	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	extgogit "github.com/fluxcd/go-git/v5"
	gitconfig "github.com/fluxcd/go-git/v5/config"
	"github.com/fluxcd/go-git/v5/plumbing"
	"github.com/fluxcd/go-git/v5/plumbing/object"
	"github.com/fluxcd/go-git/v5/plumbing/transport/http"
	helmv2beta1 "github.com/fluxcd/helm-controller/api/v2beta1"
	automationv1beta1 "github.com/fluxcd/image-automation-controller/api/v1beta1"
	reflectorv1beta1 "github.com/fluxcd/image-reflector-controller/api/v1beta1"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	notiv1beta1 "github.com/fluxcd/notification-controller/api/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/gogit"
	"github.com/fluxcd/pkg/git/repository"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
)

const defaultBranch = "main"

func setupScheme() error {
	err := sourcev1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	err = kustomizev1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	err = helmv2beta1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	err = reflectorv1beta1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	err = automationv1beta1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	err = notiv1beta1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}

	return nil
}

// fluxConfig contains configuration for installing FLux in a cluster
type installArgs struct {
	kubeconfigPath string
	repoURL        string
	password       string
	secretData     map[string]string
	kustomizeYaml  string
}

// installFlux adds the core Flux components to the cluster specified in the kubeconfig file.
func installFlux(ctx context.Context, kubeClient client.Client, conf installArgs) error {
	// Create flux-system namespace
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "flux-system",
		},
	}
	err := testEnv.Client.Create(ctx, &namespace)

	// Create additional objects that are needed for flux to run correctly
	if conf.secretData != nil {
		envCreds := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "env-creds", Namespace: "flux-system"}}
		_, err = controllerutil.CreateOrUpdate(ctx, kubeClient, envCreds, func() error {
			envCreds.StringData = conf.secretData
			return nil
		})
		if err != nil {
			return err
		}
	}

	kustomizeYaml := `
resources:
- gotk-components.yaml
- gotk-sync.yaml
`
	if conf.kustomizeYaml != "" {
		kustomizeYaml = conf.kustomizeYaml
	}

	files := make(map[string]io.Reader)
	files["./clusters/e2e/flux-system/kustomization.yaml"] = strings.NewReader(kustomizeYaml)
	files["./clusters/e2e/flux-system/gotk-components.yaml"] = strings.NewReader("")
	files["./clusters/e2e/flux-system/gotk-sync.yaml"] = strings.NewReader("")

	repo, _, err := getRepository(conf.repoURL, defaultBranch, true, conf.password)
	err = commitAndPushAll(repo, files, defaultBranch)
	if err != nil {
		return err
	}

	bootstrapCmd := fmt.Sprintf("flux bootstrap git  --url=%s --password=%s --kubeconfig=%s"+
		" --token-auth --path=clusters/e2e  --components-extra image-reflector-controller,image-automation-controller",
		conf.repoURL, conf.password, conf.kubeconfigPath)
	if err := runCommand(context.Background(), 15*time.Minute, "./", bootstrapCmd); err != nil {
		return err
	}

	return nil
}

// verifyGitAndKustomization checks that the gitrespository and kustomization combination are working properly.
func verifyGitAndKustomization(ctx context.Context, kubeClient client.Client, namespace, name string) error {
	nn := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	source := &sourcev1.GitRepository{}
	err := kubeClient.Get(ctx, nn, source)
	if err != nil {
		return err
	}
	if apimeta.IsStatusConditionPresentAndEqual(source.Status.Conditions, meta.ReadyCondition, metav1.ConditionTrue) == false {
		return fmt.Errorf("source condition not ready")
	}
	kustomization := &kustomizev1.Kustomization{}
	err = kubeClient.Get(ctx, nn, kustomization)
	if err != nil {
		return err
	}
	if apimeta.IsStatusConditionPresentAndEqual(kustomization.Status.Conditions, meta.ReadyCondition, metav1.ConditionTrue) == false {
		return fmt.Errorf("kustomization condition not ready")
	}
	return nil
}

type nsConfig struct {
	repoURL       string
	protocol      string
	objectName    string
	path          string
	modifyGitSpec func(spec *sourcev1.GitRepositorySpec)
	modifyKsSpec  func(spec *kustomizev1.KustomizationSpec)
}

// setupNamespaces creates the namespace, then creates the git secret,
// git repository and kustomization in that namespace
func setupNamespace(ctx context.Context, name string, opts nsConfig) error {
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, testEnv.Client, &namespace, func() error {
		return nil
	})

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "git-credentials",
			Namespace: name,
		},
	}

	secretData := map[string]string{
		"username": "git",
		"password": cfg.gitPat,
	}
	if opts.protocol == "ssh" {
		secretData = map[string]string{
			"identity":     cfg.gitPrivateKey,
			"identity.pub": cfg.gitPublicKey,
			"known_hosts":  cfg.knownHosts,
		}
	}

	_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, &secret, func() error {
		secret.StringData = secretData
		return nil
	})

	gitSpec := &sourcev1.GitRepositorySpec{
		Interval: metav1.Duration{
			Duration: 1 * time.Minute,
		},
		Reference: &sourcev1.GitRepositoryRef{
			Branch: name,
		},
		SecretRef: &meta.LocalObjectReference{
			Name: secret.Name,
		},
		URL: opts.repoURL,
	}
	if infraOpts.Provider == "azure" {
		gitSpec.GitImplementation = sourcev1.LibGit2Implementation
	}
	if opts.modifyGitSpec != nil {
		opts.modifyGitSpec(gitSpec)
	}
	source := &sourcev1.GitRepository{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace.Name}}
	_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, source, func() error {
		source.Spec = *gitSpec
		return nil
	})
	if err != nil {
		return err
	}

	ksSpec := &kustomizev1.KustomizationSpec{
		Path:            opts.path,
		TargetNamespace: name,
		SourceRef: kustomizev1.CrossNamespaceSourceReference{
			Kind:      sourcev1.GitRepositoryKind,
			Name:      source.Name,
			Namespace: source.Namespace,
		},
		Interval: metav1.Duration{
			Duration: 1 * time.Minute,
		},
		Prune: true,
	}
	if opts.modifyKsSpec != nil {
		opts.modifyKsSpec(ksSpec)
	}
	kustomization := &kustomizev1.Kustomization{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace.Name}}
	_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, kustomization, func() error {
		kustomization.Spec = *ksSpec
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func addFile(dir, path, content string) error {
	err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0777)
	if err != nil {
		return err
	}
	return nil
}

func commitAndPushAll(client *gogit.Client, files map[string]io.Reader, branchName string) error {
	repo, err := extgogit.PlainOpen(client.Path())
	if err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	err = wt.Checkout(&extgogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branchName),
		Force:  true,
	})
	if err != nil {
		return err
	}

	f := repository.WithFiles(files)
	_, err = client.Commit(git.Commit{
		Author: git.Signature{
			Name:  "git",
			Email: "test@example.com",
			When:  time.Now(),
		},
		Message: "add file",
	}, f)

	if err != nil {
		return err
	}

	err = client.Push(context.Background())
	if err != nil {
		return err
	}

	return nil
}

func createTagAndPush(client *gogit.Client, branchName, newTag, password string) error {
	repo, err := extgogit.PlainOpen(client.Path())
	if err != nil {
		return err
	}

	ref, err := repo.Reference(plumbing.NewBranchReferenceName(branchName), false)
	if err != nil {
		return err
	}

	tags, err := repo.TagObjects()
	if err != nil {
		return err
	}

	err = tags.ForEach(func(tag *object.Tag) error {
		if tag.Name == newTag {
			err = repo.DeleteTag(tag.Name)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	sig := &object.Signature{
		Name:  "git",
		Email: "test@example.com",
		When:  time.Now(),
	}

	_, err = repo.CreateTag(newTag, ref.Hash(), &extgogit.CreateTagOptions{
		Tagger:  sig,
		Message: "create tag",
	})
	if err != nil {
		return err
	}

	auth := &http.BasicAuth{
		Username: "git",
		Password: password,
	}

	po := &extgogit.PushOptions{
		RemoteName: "origin",
		Progress:   os.Stdout,
		RefSpecs:   []gitconfig.RefSpec{gitconfig.RefSpec("refs/tags/*:refs/tags/*")},
		Auth:       auth,
	}
	if err := repo.Push(po); err != nil {
		return err
	}

	return nil
}

func getRepository(repoURL, branchName string, overrideBranch bool, password string) (*gogit.Client, string, error) {
	checkoutBranch := defaultBranch
	if overrideBranch == false {
		checkoutBranch = branchName
	}

	tmpDir, err := os.MkdirTemp("", "*-repository")
	if err != nil {
		return nil, "", err
	}
	c, err := gogit.NewClient(tmpDir, &git.AuthOptions{
		Transport: git.HTTPS,
		Username:  "git",
		Password:  password,
	})
	if err != nil {
		return nil, "", err
	}

	_, err = c.Clone(context.Background(), repoURL, repository.CloneOptions{
		CheckoutStrategy: repository.CheckoutStrategy{
			Branch: checkoutBranch,
		},
	})

	err = c.SwitchBranch(context.Background(), branchName)
	if err != nil {
		return nil, "", err
	}

	return c, tmpDir, nil
}

func runCommand(ctx context.Context, timeout time.Duration, dir, command string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, "bash", "-c", command)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failure to run command %s: %v", string(output), err)
	}
	return nil
}

func pushImagesFromURL(repoURL, imgURL string, tags []string) error {
	img, err := crane.Pull(imgURL)
	if err != nil {
		return err
	}

	opts := crane.WithAuth(authn.FromConfig(authn.AuthConfig{
		Username: cfg.dockerCred.username,
		Password: cfg.dockerCred.password,
	}))

	for _, tag := range tags {
		log.Printf("Pushing '%s' to '%s:%s'\n", imgURL, repoURL, tag)
		if err := crane.Push(img, fmt.Sprintf("%s:%s", repoURL, tag), opts); err != nil {
			return err
		}
	}

	return nil
}
