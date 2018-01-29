# Sub site

You can host multiple sites using daptin. A sub site is exposing a cloud storage folder statically under a sub-domain, domain or a path. 

## Creating a new sub-site

- Goto dashboard https://dashboard.domain.com/
- Click "Sub sites"
- Click the green "+" icon
- Type in the **hostname** this should be exposed to
  - this can be a domain or a sub-domain
  - the domain should be pointing to the daptin instance
- Choose a **name**
- **Path**: select a sub directory name to expose this sub-site. Your sub-site will be accessible at domain.com/<path>
- **Cloud store Id**: choose an existing [cloud store](/cloudstore/cloudstore.md).

Restart to enable serving the site. 

Daptin will sync the cloud store locally and start serving it under the domain/path.