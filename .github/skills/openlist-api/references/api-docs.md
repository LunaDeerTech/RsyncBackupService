# OpenList API Docs Quick Reference

This file is derived from `https://fox.oplist.org/llms.txt`, but all concrete documentation links have already been normalized to `https://fox.oplist.org`.

Use this file instead of reading `llms.txt` directly.

## Authentication

- [User login](https://fox.oplist.org/364155678e0.md): Authenticate user with username and password, returns JWT token
- [User login with pre-hashed password](https://fox.oplist.org/364155679e0.md): Authenticate using username and pre-hashed password (SHA256)
- [LDAP login](https://fox.oplist.org/364155680e0.md): Authenticate user via LDAP directory service
- [User logout](https://fox.oplist.org/364155681e0.md): Invalidate current session token
- [Generate 2FA secret](https://fox.oplist.org/364155682e0.md): Generate a new 2FA (TOTP) secret for current user
- [Verify and enable 2FA](https://fox.oplist.org/364155683e0.md): Verify TOTP code and enable 2FA for current user
- [SSO login redirect](https://fox.oplist.org/364155684e0.md): Redirect to configured SSO provider for authentication
- [SSO callback handler](https://fox.oplist.org/364155685e0.md): Handle callback from SSO provider after authentication
- [Begin WebAuthn login](https://fox.oplist.org/364155686e0.md): Initiate WebAuthn (FIDO2/Passkey) authentication flow
- [Finish WebAuthn login](https://fox.oplist.org/364155687e0.md): Complete WebAuthn authentication with signed challenge
- [Begin WebAuthn registration](https://fox.oplist.org/364155688e0.md): Start registering a new WebAuthn credential (requires authentication)
- [Finish WebAuthn registration](https://fox.oplist.org/364155689e0.md): Complete WebAuthn credential registration
- [Delete WebAuthn credential](https://fox.oplist.org/364155690e0.md): Remove a registered WebAuthn credential
- [Get WebAuthn credentials](https://fox.oplist.org/364155691e0.md): List all registered WebAuthn credentials for current user

## User

- [Get current user info](https://fox.oplist.org/364155692e0.md): Retrieve information about the currently authenticated user
- [Update current user](https://fox.oplist.org/364155693e0.md): Update password or other settings for current user
- [List my SSH public keys](https://fox.oplist.org/364155694e0.md): Get list of SSH public keys for current user
- [Add SSH public key](https://fox.oplist.org/364155695e0.md): Add a new SSH public key for current user
- [Delete SSH public key](https://fox.oplist.org/364155696e0.md): Remove an SSH public key from current user

## Admin

- [List all users (Admin)](https://fox.oplist.org/364155697e0.md): Get paginated list of all users
- [Get user by ID (Admin)](https://fox.oplist.org/364155698e0.md): Retrieve specific user information
- [Create new user (Admin)](https://fox.oplist.org/364155699e0.md): Create a new user account
- [Update user (Admin)](https://fox.oplist.org/364155700e0.md): Update user information
- [Delete user (Admin)](https://fox.oplist.org/364155701e0.md): Delete a user account
- [Cancel user 2FA (Admin)](https://fox.oplist.org/364155702e0.md): Disable 2FA for a specific user
- [Clear user cache (Admin)](https://fox.oplist.org/364155703e0.md): Clear cached data for a specific user
- [List user SSH keys (Admin)](https://fox.oplist.org/364155704e0.md): Get SSH keys for any user
- [Delete user SSH key (Admin)](https://fox.oplist.org/364155705e0.md): Delete SSH key for any user
- [List all storages (Admin)](https://fox.oplist.org/364155706e0.md): Get list of all configured storage mounts
- [Get storage by ID (Admin)](https://fox.oplist.org/364155707e0.md): Retrieve specific storage configuration
- [Create storage (Admin)](https://fox.oplist.org/364155708e0.md): Add a new storage mount
- [Update storage (Admin)](https://fox.oplist.org/364155709e0.md): Modify existing storage configuration
- [Delete storage (Admin)](https://fox.oplist.org/364155710e0.md): Remove a storage mount
- [Enable storage (Admin)](https://fox.oplist.org/364155711e0.md): Enable a disabled storage mount
- [Disable storage (Admin)](https://fox.oplist.org/364155712e0.md): Temporarily disable a storage mount
- [Reload all storages (Admin)](https://fox.oplist.org/364155713e0.md): Force reload all storage mounts
- [List all drivers (Admin)](https://fox.oplist.org/364155714e0.md): Get list of available storage drivers with their configurations
- [Get driver names (Admin)](https://fox.oplist.org/364155715e0.md): Get list of available driver names
- [Get driver info (Admin)](https://fox.oplist.org/364155716e0.md): Get configuration schema for a specific driver
- [List all settings (Admin)](https://fox.oplist.org/364155717e0.md): Get all system settings
- [Get setting by key (Admin)](https://fox.oplist.org/364155718e0.md): Retrieve a specific setting value
- [Save settings (Admin)](https://fox.oplist.org/364155719e0.md): Update one or more system settings
- [Delete setting (Admin)](https://fox.oplist.org/364155720e0.md): Remove a custom setting
- [Reset API token (Admin)](https://fox.oplist.org/364155721e0.md): Generate a new API token
- [List all metas (Admin)](https://fox.oplist.org/364155722e0.md): Get list of metadata configurations
- [Get meta by ID (Admin)](https://fox.oplist.org/364155723e0.md): Retrieve specific metadata configuration
- [Create meta (Admin)](https://fox.oplist.org/364155724e0.md): Add new metadata configuration
- [Update meta (Admin)](https://fox.oplist.org/364155725e0.md): Modify metadata configuration
- [Delete meta (Admin)](https://fox.oplist.org/364155726e0.md): Remove metadata configuration
- [Build search index (Admin)](https://fox.oplist.org/364155727e0.md): Build full-text search index for all storages
- [Update search index (Admin)](https://fox.oplist.org/364155728e0.md): Update search index for specific paths
- [Stop indexing (Admin)](https://fox.oplist.org/364155729e0.md): Stop current indexing operation
- [Clear search index (Admin)](https://fox.oplist.org/364155730e0.md): Delete all search index data
- [Get indexing progress (Admin)](https://fox.oplist.org/364155731e0.md): Check current indexing operation progress

## File System

- [List directory contents](https://fox.oplist.org/364155732e0.md): Get list of files and directories at specified path with pagination
- [Get file or directory info](https://fox.oplist.org/364155733e0.md): Retrieve metadata for a specific file or directory
- [Search files and directories](https://fox.oplist.org/364155734e0.md): Search for files/folders by name (requires search index to be built)
- [Get directory tree](https://fox.oplist.org/364155735e0.md): Retrieve directory structure (folders only) for navigation
- [Get additional file operations](https://fox.oplist.org/364155736e0.md): Retrieve provider-specific operations available for a file/folder
- [Create directory](https://fox.oplist.org/364155737e0.md): Create a new directory at specified path
- [Rename file or directory](https://fox.oplist.org/364155738e0.md): Rename a file or directory
- [Batch rename files](https://fox.oplist.org/364155739e0.md): Rename multiple files using a pattern
- [Regex-based rename](https://fox.oplist.org/364155740e0.md): Rename files using regular expression pattern matching
- [Move files or directories](https://fox.oplist.org/364155741e0.md): Move one or more files/folders to another location
- [Recursive move](https://fox.oplist.org/364155742e0.md): Move files/folders recursively (preserves directory structure)
- [Copy files or directories](https://fox.oplist.org/364155743e0.md): Copy one or more files/folders to another location
- [Remove files or directories](https://fox.oplist.org/364155744e0.md): Delete one or more files or folders
- [Remove empty directories](https://fox.oplist.org/364155745e0.md): Recursively remove empty directories
- [Upload file (stream)](https://fox.oplist.org/364155746e0.md): Upload file using streaming (for large files or programmatic uploads)
- [Upload file (form)](https://fox.oplist.org/364155747e0.md): Upload file using multipart form data (for browser uploads)
- [Add offline download task](https://fox.oplist.org/364155748e0.md): Create an offline download task (HTTP/magnet/torrent)
- [Decompress archive](https://fox.oplist.org/364155749e0.md): Extract archive file (zip/rar/7z/tar/tar.gz etc.)
- [Get archive metadata](https://fox.oplist.org/364155750e0.md): Retrieve metadata of archive file without extracting
- [List archive contents](https://fox.oplist.org/364155751e0.md): List files inside an archive without extracting

## Public

- [Get public settings](https://fox.oplist.org/364155752e0.md): Retrieve public system settings (no authentication required)
- [Get available offline download tools](https://fox.oplist.org/364155753e0.md): List configured offline download tools (aria2, qbittorrent, etc.)
- [Get supported archive extensions](https://fox.oplist.org/364155754e0.md): List archive file extensions that can be extracted

## Sharing

- [List all shares](https://fox.oplist.org/364155755e0.md): Get paginated list of file shares created by current user
- [Get share by ID](https://fox.oplist.org/364155756e0.md): Retrieve specific share information
- [Create file share](https://fox.oplist.org/364155757e0.md): Create a new shareable link for files/folders
- [Update share](https://fox.oplist.org/364155758e0.md): Modify existing share configuration
- [Delete share](https://fox.oplist.org/364155759e0.md): Remove a file share
- [Enable share](https://fox.oplist.org/364155760e0.md): Re-enable a disabled share
- [Disable share](https://fox.oplist.org/364155761e0.md): Temporarily disable a share

## TS Version Docs

- [文件操作接口](https://fox.oplist.org/349162289e0.md): TS 版本接口文档
- [用户操作接口](https://fox.oplist.org/349175228e0.md): TS 版本接口文档
- [挂载管理接口](https://fox.oplist.org/349175254e0.md): TS 版本接口文档