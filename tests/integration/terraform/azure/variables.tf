variable "azure_devops_org" {
  type        = string
  description = "Name of Azure DevOps organizations were the repositories will be created"
}

variable "azure_location" {
  type        = string
  description = "Location of the resource group"
  default     = "eastus"
}

variable "azure_tags" {
  type        = map(string)
  default     = {}
  description = "Tags for created Azure resources"
}

variable "azuredevops_pat" {
  type        = string
  description = "Personal access token for Azure DevOps repository"
}
