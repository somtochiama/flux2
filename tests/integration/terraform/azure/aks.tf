module "aks" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/azure/aks"

  name     = "aks-${local.name_suffix}"
  location = var.azure_location
}

module "acr" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/azure/acr"

  name             = "acrapps${random_pet.suffix.id}"
  location         = var.azure_location
  aks_principal_id = [module.aks.principal_id]
  resource_group   = module.aks.resource_group

  depends_on = [module.aks]
}
ss~~