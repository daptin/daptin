# Daptin Server Source Code Analysis Tracker

**Total Files:** 176
**Progress:** 164/176 (93.2%)

## Status Legend
- 🔄 In Progress
- ✅ Completed  
- ⏳ Pending

## File Analysis Progress

### action_provider/
- ✅ `server/action_provider/action_provider.go`

### actionresponse/
- ✅ `server/actionresponse/action_pojo.go`

### actions/ (52 files)
- ✅ `server/actions/action_become_admin.go`
- ✅ `server/actions/action_cloudstore_file_delete.go`
- ✅ `server/actions/action_cloudstore_file_upload.go`
- ✅ `server/actions/action_cloudstore_folder_create.go`
- ✅ `server/actions/action_cloudstore_path_move.go`
- ✅ `server/actions/action_cloudstore_site_create.go`
- ✅ `server/actions/action_column_sync_storage.go`
- ✅ `server/actions/action_csv_to_entity.go`
- ✅ `server/actions/action_delete_column.go`
- ✅ `server/actions/action_delete_table.go`
- ✅ `server/actions/action_download_cms_config.go`
- ✅ `server/actions/action_enable_graphql.go`
- ✅ `server/actions/action_execute_process.go`
- ✅ `server/actions/action_export_csv_data.go`
- ✅ `server/actions/action_export_data.go`
- ✅ `server/actions/action_generate_acme_tls_certificate.go`
- ✅ `server/actions/action_generate_jwt_token.go`
- ✅ `server/actions/action_generate_oauth2_token.go`
- ✅ `server/actions/action_generate_password_reset_flow.go`
- ✅ `server/actions/action_generate_password_reset_verify_flow.go`
- ✅ `server/actions/action_generate_random_data.go`
- ✅ `server/actions/action_generate_self_tls_certificate.go`
- ✅ `server/actions/action_import_cloudstore_files.go`
- ✅ `server/actions/action_import_data.go`
- ✅ `server/actions/action_integration_execute.go`
- ✅ `server/actions/action_integration_install.go`
- ✅ `server/actions/action_mail_send_ses.go`
- ✅ `server/actions/action_mail_send.go`
- ✅ `server/actions/action_mail_servers_sync.go`
- ✅ `server/actions/action_make_response.go`
- ✅ `server/actions/action_network_request.go`
- ✅ `server/actions/action_oauth_login_begin.go`
- ✅ `server/actions/action_oauth_login_response.go`
- ✅ `server/actions/action_oauth_profile_exchange.go`
- ✅ `server/actions/action_otp_generate.go`
- ✅ `server/actions/action_otp_login_verify.go`
- ✅ `server/actions/action_random_value_generate.go`
- ✅ `server/actions/action_rename_column.go`
- ✅ `server/actions/action_render_template.go`
- ✅ `server/actions/action_restart_system.go`
- ✅ `server/actions/action_site_file_get.go`
- ✅ `server/actions/action_site_file_list.go`
- ✅ `server/actions/action_site_sync_storage.go`
- ✅ `server/actions/action_switch_session_user.go`
- ✅ `server/actions/action_transaction.go`
- ✅ `server/actions/action_xls_to_entity.go`
- ✅ `server/actions/json.go`
- ✅ `server/actions/streaming_export_writers.go`
- ✅ `server/actions/streaming_import_parsers.go`
- ✅ `server/actions/utils.go`

### apiblueprint/
- ✅ `server/apiblueprint/apiblueprint.go`

### assetcachepojo/
- ✅ `server/assetcachepojo/asset_cache.go`

### auth/
- ✅ `server/auth/auth_test.go`
- ✅ `server/auth/auth.go`

### cache/
- ✅ `server/cache/cached_file.go`
- ✅ `server/cache/file_cache.go`
- ✅ `server/cache/utils.go`

### cloud_store/
- ✅ `server/cloud_store/cloud_store.go`
- ✅ `server/cloud_store/utils.go`

### columns/
- ✅ `server/columns/columns.go`

### columntypes/
- ✅ `server/columntypes/mtime.go`
- ✅ `server/columntypes/types.go`

### constants/
- ✅ `server/constants/constants.go`

### csvmap/
- ✅ `server/csvmap/csvmap_test.go`
- ✅ `server/csvmap/csvmap.go`

### database/
- ✅ `server/database/database_connection_interface.go`

### dbresourceinterface/
- ✅ `server/dbresourceinterface/credential.go`
- ✅ `server/dbresourceinterface/interface.go`

### fakerservice/
- ✅ `server/fakerservice/faker_test.go`
- ✅ `server/fakerservice/faker.go`

### fsm/
- ✅ `server/fsm/fsm_manager.go`

### hostswitch/
- ✅ `server/hostswitch/host_switch.go`
- ✅ `server/hostswitch/utils.go`

### id/
- ✅ `server/id/id.go`

### jwt/
- ✅ `server/jwt/jwtmiddleware.go`

### permission/
- ✅ `server/permission/permission_test.go`
- ✅ `server/permission/permission.go`

### resource/ (42 files) - **CRITICAL SECURITY FILES ANALYZED**
- ✅ `server/resource/resource.go` - **CRITICAL: Type assertion and reflection vulnerabilities**
- ✅ `server/resource/dbresource.go` - **CRITICAL: Environment injection, UUID conversion, transaction leaks**  
- ✅ `server/resource/credentials.go` - **CRITICAL: Credential security vulnerabilities**
- ✅ `server/resource/encryption_decryption.go` - **HIGH: Cryptographic implementation gaps**
- ✅ `server/resource/action_handler_map.go` - **LOW: Global mutable state and missing thread safety**
- ✅ `server/resource/actions.go` - **HIGH: UUID parsing panic, missing binary validation**
- ✅ `server/resource/bcrypt_utils.go` - **MEDIUM: Fixed cost factor and missing input validation**
- ✅ `server/resource/certificate_manager.go` - **CRITICAL: Unsafe type assertion, CA certificate violations, private key exposure**
- ✅ `server/resource/cms_config.go` - **HIGH: SQL injection, global validator vulnerability, cache poisoning**
- ✅ `server/resource/column_types.go` - **CRITICAL: MD5 password hashing, weak random generation, ignored cryptographic errors**
- ✅ `server/resource/columns_test.go` - **MEDIUM: Missing imports, undefined dependencies, information disclosure**
- ✅ `server/resource/columns.go` - **HIGH: JSON injection, weak password validation, credential exposure, cryptographic material handling**
- ✅ `server/resource/constants.go` - **LOW: Predictable database schema names, missing documentation**
- ✅ `server/resource/credentials.go` - **CRITICAL: Multiple unsafe type assertions, ignored encryption errors, missing validation**
- ✅ `server/resource/dbfunctions_check.go` - **HIGH: SQL injection, unsafe type assertions, incomplete error handling**
- ✅ `server/resource/dbfunctions_create.go` - **CRITICAL: SQL injection through DDL statements, transaction corruption, overly permissive permissions**
- ✅ `server/resource/dbfunctions_get.go` - **CRITICAL: Unsafe type assertions, cache integrity vulnerabilities, OAuth token management flaws**
- ✅ `server/resource/dbfunctions_update.go` - **CRITICAL: File path traversal, unsafe type assertions, predictable admin credentials, data import vulnerabilities**
- ✅ `server/resource/dbmethods.go` - **CRITICAL: Extensive unsafe type assertions, cache integrity issues, admin privilege escalation, permission bypass vulnerabilities**
- ✅ `server/resource/dbresource.go` - **CRITICAL: Environment injection, unsafe type assertions, OAuth token storage, admin identification**
- ✅ `server/resource/encryption_decryption.go` - **HIGH: Base64 decode error ignored, insufficient validation, no key validation**
- ✅ `server/resource/event_create.go` - **LOW: Missing input validation, type definition not visible, missing error handling**
- ✅ `server/resource/event_delete.go` - **LOW: Missing input validation, type definition not visible, missing error handling, code duplication**
- ✅ `server/resource/event_update.go` - **LOW: Missing input validation, type definition not visible, missing error handling, code duplication, missing operation context**
- ✅ `server/resource/exchange_action.go` - **CRITICAL: Unsafe type assertions, user impersonation without authorization, SQL injection, privileged action execution**
- ✅ `server/resource/exchange_rest.go` - **CRITICAL: Unsafe type assertions, SSRF vulnerability, code injection, sensitive data exposure**
- ✅ `server/resource/exchange.go` - **HIGH: Unsafe JSON unmarshaling, exchange target type without validation, contract data without validation**
- ✅ `server/resource/fsm.go` - **LOW: Empty implementation file, missing documentation**
- ✅ `server/resource/handle_action_function_map.go` - **HIGH: Weak MD5 hash function, JSON processing without validation, AES key validation missing**
- ✅ `server/resource/handle_action.go` - **CRITICAL: Arbitrary JavaScript execution, unsafe type assertions, user switching without authorization, file operations without validation**
- ✅ `server/resource/imap_backend.go` - **HIGH: Unsafe type assertions, transaction management issues, MD5 authentication code in comments**
- ⏳ `server/resource/imap_mailbox.go`
- ✅ `server/resource/imap_user.go`
- ✅ `server/resource/mail_functions.go`
- ✅ `server/resource/middleware_datavalidation.go`
- ✅ `server/resource/middleware_eventgenerator.go`
- ✅ `server/resource/middleware_exchangegenerator.go`
- ✅ `server/resource/middleware_objectaccess_permission.go`
- ✅ `server/resource/middleware_tableaccess_permission.go`
- ✅ `server/resource/middleware_yjsgenerator.go`
- ✅ `server/resource/middlewares.go`
- ✅ `server/resource/oauth_server.go`
- ✅ `server/resource/paginated_dbmethods.go`
- ✅ `server/resource/reserved_words.go`
- ✅ `server/resource/resource_aggregate.go`
- ✅ `server/resource/resource_create.go`
- ✅ `server/resource/resource_delete.go`
- ✅ `server/resource/resource_findallpaginated.go`
- ⏳ `server/resource/resource_findone.go`
- ⏳ `server/resource/resource_update.go`
- ⏳ `server/resource/resource.go`
- ⏳ `server/resource/response.go`
- ⏳ `server/resource/storage.go`
- ⏳ `server/resource/streams.go`
- ⏳ `server/resource/task_scheduler.go`
- ⏳ `server/resource/task_sync_storage.go`
- ⏳ `server/resource/translations.go`
- ⏳ `server/resource/user.go`
- ⏳ `server/resource/utils.go`

### rootpojo/
- ✅ `server/rootpojo/cloud_store.go`
- ✅ `server/rootpojo/data_import_file.go`

### statementbuilder/
- ✅ `server/statementbuilder/statement_builder.go` - **LOW: Global mutable state and missing input validation**

### subsite/
- ✅ `server/subsite/subsite_staticfs_server.go` - **CRITICAL: Path traversal vulnerabilities**
- ✅ `server/subsite/template_handler.go` - **CRITICAL: Multiple injection vulnerabilities**
- ✅ `server/subsite/utils.go` - **HIGH: Type assertion and log injection**
- ✅ `server/subsite/get_all_subsites.go` - **MEDIUM: Data integrity and performance issues**
- ✅ `server/subsite/subsite_action_config.go` - **HIGH: Type assertion and JSON injection**
- ✅ `server/subsite/subsite_cache_config.go` - **HIGH: Configuration manipulation vulnerabilities**

### table_info/
- ✅ `server/table_info/tableinfo.go`

### task/
- ✅ `server/task/task.go`

### task_scheduler/
- ✅ `server/task_scheduler/task_scheduler.go`

### websockets/
- ✅ `server/websockets/web_socket_connection_handler.go` - **CRITICAL: Multiple type assertion and permission bypass vulnerabilities**
- ✅ `server/websockets/websocket_client.go` - **CRITICAL: Type assertion and resource management vulnerabilities**
- ✅ `server/websockets/websocket_server.go` - **HIGH: Client management and security vulnerabilities**

### Root server/ files
- ✅ `server/asset_column_sync.go` - **CRITICAL: Environment injection and unsafe task scheduling vulnerabilities**
- ✅ `server/asset_presigned_url.go` - **CRITICAL: Credential exposure and multiple injection vulnerabilities**
- ✅ `server/asset_route_handler.go` - **CRITICAL: Path traversal and type assertion vulnerabilities**
- ✅ `server/asset_upload_handler.go` - **CRITICAL: Multiple upload vulnerabilities and credential exposure**
- ✅ `server/assets_column_handler.go` - **HIGH: Global state synchronization and dependency injection vulnerabilities**
- ✅ `server/banner.go` - **LOW: Information disclosure through application banner**
- ✅ `server/config_handler.go` - **CRITICAL: Unsafe type assertion and missing input validation for configuration management**
- ✅ `server/config.go` - **CRITICAL: Path traversal and environment injection vulnerabilities in configuration loading**
- ✅ `server/cors.go` - **CRITICAL: Complete CORS security bypass with permissive configuration and origin reflection**
- ✅ `server/database_connection.go` - **HIGH: Connection string injection and environment variable vulnerabilities**
- ✅ `server/endpoint_caldav.go` - **CRITICAL: Path traversal and insufficient access control in WebDAV implementation**
- ✅ `server/endpoint_favicon.go` - **MEDIUM: Resource management and validation issues in favicon serving**
- ✅ `server/endpoint_ftp_init.go` - **CRITICAL: Insecure FTP server defaults and missing validation**
- ✅ `server/endpoint_ftp.go` - **CRITICAL: Missing validation and resource access control in FTP server creation**
- ✅ `server/endpoint_graphql.go` - **CRITICAL: Missing authentication and development features exposed in GraphQL**
- ✅ `server/endpoint_imap.go` - **CRITICAL: Missing validation and insecure defaults in IMAP server initialization**
- ✅ `server/endpoint_init.go` - **HIGH: Transaction management and validation issues in server initialization**
- ✅ `server/endpoint_no_route.go` - **CRITICAL: Path traversal and cache manipulation vulnerabilities**
- ✅ `server/endpoint_yjs.go` - **CRITICAL: Authentication bypass and resource management vulnerabilities**
- ✅ `server/event_message_handler.go` - **CRITICAL: Type assertion vulnerabilities and Redis message injection**
- ✅ `server/feed_handler.go` - **CRITICAL: Multiple type assertion vulnerabilities and content injection**
- ✅ `server/file_serving_utils.go` - **HIGH: Path traversal vulnerability and memory exhaustion issues**
- ✅ `server/ftp_server.go` - **CRITICAL: Path traversal, authentication vulnerabilities, and resource management issues**
- ✅ `server/graphql.go` - **CRITICAL: Multiple type assertion vulnerabilities, SQL injection, and authorization bypass**
- ✅ `server/handlers.go` - **CRITICAL: Authentication bypass, SQL injection, and state machine security vulnerabilities**
- ✅ `server/image.go` - **CRITICAL: Resource exhaustion and memory exhaustion vulnerabilities in image processing**
- ✅ `server/inmemory_mock_db.go` - **MEDIUM: Information disclosure and memory management issues in test environment**
- ✅ `server/jsmodel_handler.go` - **CRITICAL: Authentication bypass, SQL injection, and cache exhaustion vulnerabilities**
- ✅ `server/language.go` - **MEDIUM: Input validation and memory management issues in language middleware**
- ✅ `server/mail_adapter.go` - **CRITICAL: Multiple authentication bypass, cryptographic, and mail injection vulnerabilities**
- ✅ `server/merge_tables.go` - **LOW: Input validation and memory management issues in table configuration merging**
- ✅ `server/middleware_ratelimit.go` - **HIGH: IP spoofing vulnerability and memory exhaustion through rate limiter proliferation**
- ✅ `server/resource_methods_test.go` - **MEDIUM: Resource leaks and hardcoded credentials in test environment**
- ✅ `server/server.go` - **CRITICAL: Main server initialization with multiple critical vulnerabilities including weak JWT secrets, JSON injection, and resource leaks**
- ✅ `server/smtp_server.go` - **CRITICAL: SMTP server with world-readable private keys, unsafe type assertions, and missing certificate cleanup**
- ✅ `server/statistics.go` - **HIGH: System statistics endpoint with extensive information disclosure including processes, users, and system details without authentication**
- ✅ `server/streams_test.go` - **MEDIUM: Test with resource leaks, unsafe query patterns, and inadequate validation**
- ✅ `server/sub_path_fs.go` - **CRITICAL: Path traversal vulnerability through unsafe string concatenation enabling directory traversal attacks**
- ✅ `server/subsite_cache.go` - **HIGH: Distributed cache with memory exhaustion vulnerabilities, unsafe deserialization, and path exposure**
- ✅ `server/subsite_engine.go` - **MEDIUM: Subsite engine with unprotected statistics endpoints and information disclosure through logging**
- ✅ `server/subsite_handler.go` - **CRITICAL: Subsite handler with path traversal vulnerabilities, unsafe type assertions, and host header injection enabling cache poisoning**
- ✅ `server/subsites.go` - **HIGH: Subsites initialization with environment variable injection, unsafe task scheduling, and admin credential exposure**
- ✅ `server/utils.go` - **CRITICAL: Utility functions with weak cryptographic key generation, panic conditions, and environment variable injection vulnerabilities**
- ✅ `server/yjs_doucment_provider.go` - **HIGH: YJS document provider with path injection, insecure file permissions, and unsafe type assertions**