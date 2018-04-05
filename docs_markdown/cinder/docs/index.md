<h1>Cinder Theme <small>for MkDocs</small></h1>

## About

Cinder is a clean, responsive theme for static documentation sites that are generated with [MkDocs](https://github.com/mkdocs/mkdocs). It's built on the [Bootstrap framework](http://getbootstrap.com/) and includes pre-packaged syntax highlighting ([highlight.js](https://highlightjs.org/)), icons (<i class="fa fa-flag"></i> [FontAwesome](https://fortawesome.github.io/Font-Awesome/)), and a smashingly legible type scheme to get your message out to your users.

You are looking at the theme and can see a selection of the theme elements on the [Specimen page](/specimen/).

## Install

**<em>Required</em>**: Python 2.6+ or 3.3+

### Install MkDocs & Create a New Project

If you haven't installed MkDocs yet, use the following command to install it:

<pre><code class="nohighlight">$ pip install mkdocs</code></pre>

Next, navigate to a clean directory and create a new MkDocs project with the following command:

<pre><code class="nohighlight">$ mkdocs new [projectname]</code></pre>

Replace `[projectname]` with the name of your project (without the brackets).

Then navigate to the root of your project directory:

<pre><code class="nohighlight">$ cd [projectname]</code></pre>

### Install the Cinder Theme

Choose one of the following install approaches:

#### 1. Install with pip

If you are using MkDocs v0.15.0 or higher, you can install the Cinder theme with pip using the command:

<pre><code class="nohighlight">$ pip install mkdocs-cinder</code></pre>

MkDocs projects use a YAML settings file called `mkdocs.yml`.  This is located in the root of your project directory after you use the `mkdocs new` command.  Open the file in a text editor and modify it to define Cinder in the `theme` setting as follows (note that this is case-sensitive):

<pre><code class="yaml">site_name: [YOURPROJECT]
theme: cinder
pages:
- Home: index.md
</code></pre>

**Theme Updates**: If you choose the pip install approach, you can update your Cinder theme to new releases with the command `$ pip install --upgrade mkdocs-cinder`.  Then re-build your static site files (see instructions below).

#### 2. Manual Install

Download the Cinder theme archive.

<a href="https://github.com/chrissimpkins/cinder/archive/v0.10.0.zip"><button type="button" class="btn btn-success"><i class="fa fa-cloud-download"></i>  Download Cinder v0.10.0</button></a>

Unpack the contents of the archive into a directory named `cinder` at the top level of your MkDocs project directory.

Your project directory should now look like this:

<pre><code class="nohighlight">.
├── mkdocs.yml
├── cinder
│     ├── css
│     ├── img
│     ├── js
│     ├── base.html
│     ├── content.html
│     ├── 404.html
│     ├── nav-sub.html
│     ├── nav.html
│     └── toc.html
└── docs
      └── index.md
</code></pre>

MkDocs projects use a YAML settings file called `mkdocs.yml`.  This is located in the root of your project directory after you use the `mkdocs new` command.  Open the file in a text editor and modify it to include the `theme_dir` setting as follows:

<pre><code class="yaml">site_name: [YOURPROJECT]
theme_dir: cinder
pages:
- Home: index.md
</code></pre>

**Theme Updates**: If you choose the manual install approach, you can update your Cinder theme by downloading the new cinder.zip release archive and including it in your project. Then re-build your static site files (see instructions below).

## Test with a Local Site Server


Use the following command to establish a local server for your site:

<pre><code class="nohighlight">$ mkdocs serve</code></pre>

Then open your site in any browser at the URL `http://localhost:8000`.

## Create Your Site

### Add Content with Markdown Syntax

Get to work on your site home page by opening the `docs/index.md` file and editing it in Markdown syntax.  The HTML automatically updates in the browser when you save the Markdown file if you use the MkDocs server (see command above).

### Add New Pages

Add new pages to your site by creating a new Markdown file in your `docs` directory, then linking to the new page in the `mkdocs.yml` file.  This uses a `Page Name : Markdown file` syntax.

For example, to add an About page using a Markdown file that is located on the path `docs/about.md`, you would format the `mkdocs.yml` file as follows:

<pre><code class="yaml">site_name: [YOURPROJECT]
theme_dir: cinder
pages:
- Home: index.md
- About: about.md</code></pre>

Add additional pages to your site by repeating the above series of steps.

## Build Your Site

Build your site files with the command:

<pre><code class="nohighlight">$ mkdocs build</code></pre>

Your site files are built in the `site` directory and are ready to use.  Deploy the contents of the `site` directory to your web server.

## Site Customization

The following are a few common customizations that you might be interested in.  For much more detail about the configuration of your site, check out the [MkDocs Configuration documentation](https://github.com/mkdocs/mkdocs/blob/master/docs/user-guide/configuration.md).

### Site Favicon

Place your site favicon image file in the top level of your docs directory and then include a new `site_favicon:` field in the `mkdocs.yml` file:

<pre><code class="yaml">site_name: [YOURPROJECT]
theme_dir: cinder
site_favicon: favicon.ico
pages:
  - Home: index.md
  - About: about.md</code></pre>

### Add Your Own CSS Stylesheets

Create a `css` directory inside your `docs` directory and add your CSS files.  You can overwrite any of the Cinder styles in your CSS files.  Then include your CSS files in the `mkdocs.yml` file with the `extra_css` field:

<pre><code class="yaml">site_name: [YOURPROJECT]
theme_dir: cinder
extra_css:
  - "css/mystyle.css"
  - "css/myotherstyle.css"
pages:
  - Home: index.md
  - About: about.md</code></pre>

Your CSS styles fall at the end of the cascade and will override all styles included in the theme (including Bootstrap and default Cinder styles).  You can find the Cinder and Bootstrap CSS files on the paths `cinder/css/cinder.css` and `cinder/css/bootstrap.min.css`, respectively.


### Add Your Own JavaScript

Create a `js` directory inside your `docs` directory and add your JS files.  Then include your JS files in the `mkdocs.yml` file with the `extra_js` field:

<pre><code class="yaml">site_name: [YOURPROJECT]
theme_dir: cinder
extra_js:
  - "js/myscript.js"
  - "js/myotherscript.js"
pages:
  - Home: index.md
  - About: about.md</code></pre>

### Github or Bitbucket Repository Link

Include the `repo_url` field and define it with your repository URL:

<pre><code class="yaml">site_name: [YOURPROJECT]
theme_dir: cinder
repo_url: "https://github.com/chrissimpkins/cinder"
pages:
  - Home: index.md
  - About: about.md</code></pre>

The link appears at the upper right hand corner of your site.

### License Declaration and Link

The Cinder theme displays your license declaration in the footer if you include a `copyright` field and define it with the text (and optionally the HTML link) that you would like to display:

<pre><code class="yaml">site_name: [YOURPROJECT]
theme_dir: cinder
copyright: "Cinder is licensed under the &lt;a href='https://github.com/chrissimpkins/cinder/blob/master/LICENSE.md'&gt;MIT license</a>"
pages:
  - Home: index.md
  - About: about.md</code></pre>

## Issues

If you have any issues with the theme, please report them on the Cinder repository:

<a href="https://github.com/chrissimpkins/cinder/issues/new"><button class="btn btn-primary" type="submit">Report Issue</button></a>
<a href="https://github.com/chrissimpkins/cinder/issues"><button class="btn btn-primary" type="submit">Active Issues</button></a>

## License

Cinder is licensed under the MIT license. The full text of the license is available [here](https://github.com/chrissimpkins/cinder/blob/master/LICENSE.md).



