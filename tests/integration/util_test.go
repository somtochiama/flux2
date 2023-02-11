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
	"errors"
	"fmt"
	"github.com/fluxcd/pkg/git/gogit"
	"github.com/fluxcd/pkg/git/repository"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	gossh "golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	extgogit "github.com/fluxcd/go-git/v5"
	gitconfig "github.com/fluxcd/go-git/v5/config"
	"github.com/fluxcd/go-git/v5/plumbing"
	"github.com/fluxcd/go-git/v5/plumbing/object"
	"github.com/fluxcd/go-git/v5/plumbing/transport"
	"github.com/fluxcd/go-git/v5/plumbing/transport/http"
	"github.com/fluxcd/go-git/v5/plumbing/transport/ssh"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/ssh/knownhosts"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
)

const defaultBranch = "main"

// fluxConfig contains configuration for installing FLux in a cluster
type installArgs struct {
	kubeconfigPath string
	secretData     map[string]string
}

// installFlux adds the core Flux components to the cluster specified in the kubeconfig file.
func installFlux(ctx context.Context, kubeClient client.Client, conf installArgs) error {
	// Create flux-system namespace
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "flux-system",
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, kubeClient, &namespace, func() error {
		return nil
	})

	// Create additional secrets that are needed for flux to run correctly
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
	if cfg.kustomizationYaml != "" {
		kustomizeYaml = cfg.kustomizationYaml
	}

	files := make(map[string]io.Reader)
	files["clusters/e2e/flux-system/kustomization.yaml"] = strings.NewReader(kustomizeYaml)
	files["clusters/e2e/flux-system/gotk-components.yaml"] = strings.NewReader("")
	files["clusters/e2e/flux-system/gotk-sync.yaml"] = strings.NewReader("")

	repoURL := getTransportURL(cfg.fleetInfraRepository)
	if err != nil {
		return err
	}
	c, err := getRepository(ctx, repoURL, defaultBranch, cfg.defaultAuthOpts)
	if err != nil {
		return err
	}

	err = commitAndPushAll(ctx, c, files, defaultBranch)
	if err != nil {
		return err
	}

	var bootstrapArgs string
	if cfg.defaultGitTransport == git.SSH {
		f, err := os.CreateTemp("", "*")
		if err != nil {
			return err
		}
		err = os.WriteFile(f.Name(), []byte(cfg.gitPrivateKey), 0o644)
		if err != nil {
			return err
		}
		bootstrapArgs = fmt.Sprintf("--private-key-file=%s", f.Name())
	} else {
		bootstrapArgs = fmt.Sprintf("--token-auth --password=%s", cfg.gitPat)
	}

	bootstrapCmd := fmt.Sprintf("flux bootstrap git  --url=%s %s --kubeconfig=%s --path=clusters/e2e  --components-extra image-reflector-controller,image-automation-controller",
		repoURL, bootstrapArgs, conf.kubeconfigPath)

	if cfg.defaultGitTransport == git.SSH {
		// supply prompt
		bootstrapCmd = fmt.Sprintf("echo y | %s", bootstrapCmd)
	}

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
	protocol      git.TransportType
	objectName    string
	path          string
	modifyGitSpec func(spec *sourcev1.GitRepositorySpec)
	modifyKsSpec  func(spec *kustomizev1.KustomizationSpec)
}

// setupNamespaces creates the namespace, then creates the git secret,
// git repository and kustomization in that namespace
func setupNamespace(ctx context.Context, name string, opts nsConfig) error {
	transport := cfg.defaultGitTransport
	if opts.protocol != "" {
		transport = opts.protocol
	}

	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, testEnv.Client, &namespace, func() error {
		return nil
	})
	if err != nil {
		return err
	}

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

	if transport == git.SSH {
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
	if err != nil {
		return err
	}

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

// getRepository creates a temporary directory and clones the git repository to it.
// if the repository is empty, it initializes a new git repository
func getRepository(ctx context.Context, repoURL, branchName string, authOpts *git.AuthOptions) (*gogit.Client, error) {

	tmpDir, err := os.MkdirTemp("", "*-repository")
	if err != nil {
		return nil, err
	}

	client, err := gogit.NewClient(tmpDir, authOpts, gogit.WithSingleBranch(false), gogit.WithDiskStorage())
	if err != nil {
		return nil, err
	}

	_, err = client.Clone(ctx, repoURL, repository.CloneOptions{
		CheckoutStrategy: repository.CheckoutStrategy{
			Branch: branchName,
		},
	})
	if err != nil {
		return nil, err
	}

	return client, nil
}

func addFile(dir, path, content string) error {
	fullPath := filepath.Join(dir, path)
	err := os.MkdirAll(filepath.Dir(fullPath), 0777)
	if err != nil {
		return err
	}

	err = os.WriteFile(fullPath, []byte(content), 0777)
	if err != nil {
		return err
	}
	return nil
}

// commitAndPushAll checks out to the specified branch, creates the files, commits and then pushes them to
// the remote git repository.
func commitAndPushAll(ctx context.Context, client *gogit.Client, files map[string]io.Reader, branchName string) error {
	switchAgain := false
	err := switchBranch(client, branchName)
	if err != nil {
		// we get reference not found error when trying to check out to a new branch on empty directory
		// so we make a commit then attempt to checkout again.
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			switchAgain = true
		} else {
			return err
		}
	}

	_, err = client.Commit(git.Commit{
		Author: git.Signature{
			Name:  "git",
			Email: "test@example.com",
			When:  time.Now(),
		},
	}, repository.WithFiles(files))
	if err != nil {
		if errors.Is(err, git.ErrNoStagedFiles) {
			return nil
		}

		return err
	}

	if switchAgain {
		if err := switchBranch(client, branchName); err != nil {
			return err
		}
	}

	err = client.Push(ctx)
	if err != nil {
		return fmt.Errorf("unable to push: %s", err)
	}

	return nil
}

// SwitchBranch switches to a branch, if the branch doesn't exist and there's a remote branch with the same name,
// it creates a new branch pointing to the remote's head.
func switchBranch(client *gogit.Client, branchName string) error {
	repo, err := extgogit.PlainOpen(client.Path())
	if err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	remote, local := true, true
	remRefName := plumbing.NewRemoteReferenceName(extgogit.DefaultRemoteName, branchName)
	remRef, err := repo.Reference(remRefName, true)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		remote = false
	} else if err != nil {
		return fmt.Errorf("could not fetch remote reference '%s': %w", branchName, err)
	}

	refName := plumbing.NewBranchReferenceName(branchName)
	_, err = repo.Reference(refName, true)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		local = false
	} else if err != nil {
		return fmt.Errorf("could not fetch local reference '%s': %w", branchName, err)
	}

	create := false
	// If the remote branch exists, but not the local branch, create a local
	// reference to the remote's HEAD.
	if remote && !local {
		branchRef := plumbing.NewHashReference(refName, remRef.Hash())

		err = repo.Storer.SetReference(branchRef)
		if err != nil {
			return fmt.Errorf("could not create reference to remote HEAD '%s': %w", branchRef.Hash().String(), err)
		}
	} else if !remote && !local {
		// If the target branch does not exist locally or remotely, create a new
		// branch using the current worktree HEAD.
		create = true
	}

	err = wt.Checkout(&extgogit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branchName),
		Force:  true,
		Create: create,
	})
	if err != nil {
		return fmt.Errorf("could not checkout to branch '%s': %w", branchName, err)
	}

	return nil
}

func createTagAndPush(ctx context.Context, path, branchName, newTag string, opts *git.AuthOptions) error {
	repo, err := extgogit.PlainOpen(path)
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

	auth, err := transportAuth()
	if err != nil {
		return err
	}

	err = tags.ForEach(func(tag *object.Tag) error {
		if tag.Name == newTag {
			err = repo.DeleteTag(tag.Name)
			if err != nil {
				return err
			}

			// delete remote tag
			po := &extgogit.PushOptions{
				RemoteName: "origin",
				Progress:   os.Stdout,
				RefSpecs:   []gitconfig.RefSpec{gitconfig.RefSpec(":refs/tags/v1")},
				Auth:       auth,
				Force:      true,
			}

			if err := repo.Push(po); err != nil {
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

	po := &extgogit.PushOptions{
		RemoteName: "origin",
		Progress:   os.Stdout,
		RefSpecs:   []gitconfig.RefSpec{gitconfig.RefSpec("refs/tags/*:refs/tags/*")},
		Auth:       auth,
	}

	if err := repo.PushContext(ctx, po); err != nil {
		return err
	}

	return nil
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
	//crane.Tag()
	if err != nil {
		return err
	}

	for _, tag := range tags {
		log.Printf("Pushing '%s' to '%s:%s'\n", imgURL, repoURL, tag)
		if err := crane.Push(img, fmt.Sprintf("%s:%s", repoURL, tag)); err != nil {
			return err
		}
	}

	return nil
}

func getTransportURL(repoCfg repoConfig) string {
	if cfg.defaultGitTransport == git.SSH {
		return repoCfg.ssh
	}

	return repoCfg.http
}

func authOpts(repoURL string, authData map[string][]byte) (*git.AuthOptions, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return nil, err
	}

	authOpts, err := git.NewAuthOptions(*u, authData)
	if err != nil {
		return nil, err
	}
	return authOpts, nil
}

// transportAuth constructs the transport.AuthMethod for the default git transport in the config of
// the given git.AuthOptions. It returns the result, or an error.
func transportAuth() (transport.AuthMethod, error) {
	if cfg.defaultGitTransport == git.SSH {
		pk, err := ssh.NewPublicKeys(cfg.gitUsername, []byte(cfg.gitPrivateKey), "")
		if err != nil {
			return nil, err
		}

		clbk, err := knownhosts.New([]byte(cfg.knownHosts))
		if err != nil {
			return nil, err
		}

		return &CustomPublicKeys{
			pk:       pk,
			callback: clbk,
		}, nil
	} else {
		return &http.BasicAuth{
			Username: "git",
			Password: cfg.gitPat,
		}, nil
	}
}

// TODO: Export function from fluxcd/pkg
// Copied from: https://github.com/fluxcd/pkg/blob/main/git/gogit/transport.go
// CustomPublicKeys is a wrapper around ssh.PublicKeys to help us
// customize the ssh config. It implements ssh.AuthMethod.
type CustomPublicKeys struct {
	pk       *ssh.PublicKeys
	callback gossh.HostKeyCallback
}

func (a *CustomPublicKeys) Name() string {
	return a.pk.Name()
}

func (a *CustomPublicKeys) String() string {
	return a.pk.String()
}

func (a *CustomPublicKeys) ClientConfig() (*gossh.ClientConfig, error) {
	config, err := a.pk.ClientConfig()
	if err != nil {
		return nil, err
	}

	if a.callback != nil {
		config.HostKeyCallback = a.callback
	}
	if len(git.KexAlgos) > 0 {
		config.Config.KeyExchanges = git.KexAlgos
	}
	if len(git.HostKeyAlgos) > 0 {
		config.HostKeyAlgorithms = git.HostKeyAlgos
	}

	return config, nil
}
