resource "azurerm_key_vault" "this" {
  name                = "kv-credentials-${random_pet.suffix.id}"
  resource_group_name = azurerm_resource_group.this.name
  location            = azurerm_resource_group.this.location
  tenant_id           = data.azurerm_client_config.current.tenant_id
  sku_name            = "standard"
}

resource "azurerm_key_vault_access_policy" "admin" {
  key_vault_id = azurerm_key_vault.this.id
  tenant_id = data.azurerm_client_config.current.tenant_id
  object_id = data.azurerm_client_config.current.object_id

  key_permissions = [
    "Create",
    "Encrypt",
    "Delete",
    "Get",
    "List",
    "Purge",
    "Recover",
  ]
  
}

resource "azurerm_key_vault_access_policy" "cluster_binding" {
  key_vault_id = azurerm_key_vault.this.id
  tenant_id = data.azurerm_client_config.current.tenant_id
  object_id = azurerm_kubernetes_cluster.this.kubelet_identity[0].object_id

  key_permissions = [
    "Decrypt",
    "Encrypt",
  ]
}

resource "azurerm_key_vault_key" "sops" {
  depends_on = [azurerm_key_vault_access_policy.admin]

  name         = "sops"
  key_vault_id = azurerm_key_vault.this.id
  key_type     = "RSA"
  key_size     = 2048

  key_opts = [
    "decrypt",
    "encrypt",
  ]
}
