resource "azurerm_eventhub_namespace" "this" {
  name = "ehns-${local.name_suffix}"
  location            = var.location
  resource_group_name = module.aks.resource_group
  sku                 = "Basic"
  capacity            = 1
}


resource "azurerm_eventhub" "this" {
  name = "eh-${local.name_suffix}"
  namespace_name      = azurerm_eventhub_namespace.this.name
  resource_group_name = module.aks.resource_group
  partition_count     = 1
  message_retention   = 1
}

resource "azurerm_eventhub_authorization_rule" "this" {
  name                = "flux"
  resource_group_name = module.aks.resource_group
  namespace_name      = azurerm_eventhub_namespace.this.name
  eventhub_name       = azurerm_eventhub.this.name
  listen              = true
  send                = true
  manage              = false
}
