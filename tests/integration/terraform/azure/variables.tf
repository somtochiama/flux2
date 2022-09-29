variable "azure_devops_org" {
    type = string
    default = "flux-azure"
    description = "Name of Azure DevOps organizations were the repositories will be created"
}

variable "azuredevops_pat" {
    type = string
    description = "Personal access token for Azure DevOps repository"
}

variable "azuredevops_id_rsa" {
    type = string
    description = "Private key for ssh access to Azure DevOps repository"
}

variable "azuredevops_id_rsa_pub" {
    type = string
    description = "Public key for ssh access to Azure DevOps repository"
}
