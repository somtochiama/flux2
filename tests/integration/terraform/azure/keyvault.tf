resource "azurerm_key_vault" "this" {
  name                = "kv-credentials-${random_pet.suffix.id}"
  resource_group_name = data.azurerm_resource_group.this.name
  location            = data.azurerm_resource_group.this.location
  tenant_id           = data.azurerm_client_config.current.tenant_id
  sku_name            = "standard"
}

resource "azurerm_key_vault_access_policy" "admin" {
  key_vault_id = azurerm_key_vault.this.id
  tenant_id = data.azurerm_client_config.current.tenant_id
  object_id = data.azurerm_client_config.current.object_id

  key_permissions = [
    "Create",
    "Decrypt",
    "Delete",
    "Encrypt",
    "Get",
    "Import",
    "List",
    "Purge",
    "Recover",
  ]

  secret_permissions = [
    "Backup",
    "Delete",
    "Get",
    "List",
    "Purge",
    "Recover",
    "Restore",
    "Set",
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

resource "azurerm_key_vault_secret" "pat" {
  depends_on = [azurerm_key_vault_access_policy.admin]

  name         = "pat"
  value        = var.azuredevops_pat
  key_vault_id = azurerm_key_vault.this.id
}

resource "azurerm_key_vault_secret" "id_rsa" {
  depends_on = [azurerm_key_vault_access_policy.admin]

  name         = "id-rsa"
  value        = var.azuredevops_id_rsa
  key_vault_id = azurerm_key_vault.this.id
}

resource "azurerm_key_vault_secret" "id_rsa_pub" {
  depends_on = [azurerm_key_vault_access_policy.admin]

  name         = "id-rsa-pub"
  value        = var.azuredevops_id_rsa_pub
  key_vault_id = azurerm_key_vault.this.id
}

data "azurerm_key_vault_secret" "pat" {
  depends_on = [azurerm_key_vault_secret.pat]
  key_vault_id = resource.azurerm_key_vault.this.id
  name         = "pat"
}

data "azurerm_key_vault_secret" "id_rsa" {
  depends_on = [azurerm_key_vault_secret.id_rsa]

  key_vault_id = resource.azurerm_key_vault.this.id
  name         = "id-rsa"
}

data "azurerm_key_vault_secret" "id_rsa_pub" {
  depends_on = [azurerm_key_vault_secret.id_rsa_pub]

  key_vault_id = resource.azurerm_key_vault.this.id
  name         = "id-rsa-pub"
}
