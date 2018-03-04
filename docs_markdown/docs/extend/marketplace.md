# Marketplace

Market places are git based repositories where you can keep your collections of schemas, to be re-used by you or others later.

## Create a market place

- Create a git repository
- Goto **Dashboard**
- Click **Marketplace**
- Click PLUS icon to add a new marketplace
- Enter git endpoint
- Enter a name
- If your packages are not at the root, then enter a path to the subpackages
- Or leave this path empty

Click submit to add this. Remember to "Sync repository" once before installing a package.

Syncing makes a local clone of the git repository for usage, or pulls for changes if it exists already.

## Example of a market place git repository

[Checkout a dummy market place](https://github.com/artpar/daptin-marketplace-dummy) with a couple of packages to be used

- Blog
- Construction project management system
- FAQ management system
- Store management system
- Fashion style management system
- Todo list


## Install a package from a market place

- Goto **Dashboard**
- Click **Marketplace**
- Go into a marketplace
- Click Action "Install package"
- Type in the package name: this is the name of the folder you want to install

Submit to install this package. Daptin will restart itself and makes the changes to the APIs.