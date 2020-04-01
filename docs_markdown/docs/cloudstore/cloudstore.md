# Cloud store

Datin can work with the following storage services:

- Amazon Drive  
- Amazon S3  
- Backblaze B2  
- Box  
- Ceph  
- DigitalOcean Spaces  
- Dreamhost  
- Dropbox  
- FTP  
- Google Cloud Storage  
- Google Drive  
- HTTP  
- Hubic  
- Memset Memstore  
- Microsoft Azure Blob Storage  
- Microsoft OneDrive  
- Minio  
- Nextloud  
- OVH  
- Openstack Swift  
- Oracle Cloud Storage  
- Ownloud  
- pCloud  
- put.io  
- QingStor  
- Rackspace Cloud Files  
- SFTP  
- Wasabi  
- WebDAV  
- Yandex Disk  
- The local filesystem  

## Creating a new cloud storage instance

### Things to keep ready

If the service you wan to integrate with requires authentication, create the following:

- An [oauth connection](/extend/oauth_connection)
- An [oauth token](/extend/oauth_token) generated from the above connection

### Steps

- Login to the dashboard
- Click "Storage" tile
- Click the green "+" icon on the top right
- Use the **name** to identify it uniquely
- **Root Path**: in rclone format, eg
  - gdrive: `drive:directory/subdirectory`
  - dropbox/ftp/local: `remote/local:directory/subdirectory`
- **Store Provider**: dropbox/drive/local/ftp...
- **Store Type**: cloud/local