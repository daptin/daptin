# action_provider.go

**File:** server/action_provider/action_provider.go

## Code Summary

### Function: GetActionPerformers
**Inputs:**
- `initConfig *resource.CmsConfig` - System configuration
- `configStore *resource.ConfigStore` - Configuration storage access
- `cruds map[string]*resource.DbResource` - Database resource handlers
- `mailDaemon *guerrilla.Daemon` - Mail server daemon
- `hostSwitch hostswitch.HostSwitch` - Host switching functionality
- `certificateManager *resource.CertificateManager` - TLS certificate management

**Process:**
1. Creates database transaction using `cruds["world"].Connection().Beginx()`
2. Initializes empty `performers` slice
3. Creates 43 different action performers by calling `actions.New*` functions:
   - Line 25: `actions.NewBecomeAdminPerformer(initConfig, cruds)`
   - Line 29: `actions.NewImportCloudStoreFilesPerformer(initConfig, cruds)`
   - Line 33: `actions.NewOtpGenerateActionPerformer(cruds, configStore, transaction)`
   - [continues through line 195 creating all performers]
4. For each performer created, appends to `performers` slice
5. Calls `cruds["world"].GetActiveIntegrations(transaction)` to get dynamic integrations
6. For each active integration, creates `actions.NewIntegrationActionPerformer()`
7. Registers all performers in `resource.ActionHandlerMap[performer.Name()]`

**Output:**
- Returns `[]actionresponse.ActionPerformerInterface` containing all created performers
- Side effect: Populates global `resource.ActionHandlerMap` with performer name mappings

**Error Handling:**
- Uses `resource.CheckErr(err, "message")` after each performer creation
- If transaction creation fails, returns `nil`
- If integration performer creation fails, logs error and continues with next integration

**Database Operations:**
- Creates and commits transaction
- Queries active integrations from database