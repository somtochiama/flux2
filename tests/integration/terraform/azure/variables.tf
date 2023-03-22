variable "azure_devops_org" {
    type = string
    default = "flux-azure"
    description = "Name of Azure DevOps organizations were the repositories will be created"
}

variable "location" {
    type = string
    description = "Location of the resource group"
    default = "southcentralus"
}

variable "azuredevops_pat" {
    type = string
    description = "Personal access token for Azure DevOps repository"
}
