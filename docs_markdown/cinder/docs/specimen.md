<h1>Cinder Theme Specimen</h1>

## Typography

### Typefaces

- Headers: [PT Sans](https://www.google.com/fonts/specimen/PT+Sans)
- Body: [Open Sans](https://www.google.com/fonts/specimen/Open+Sans)
- Code: [Hack](http://sourcefoundry.org/hack/)

### Body Copy

You think water moves fast? You should see ice. <strong>It moves like it has a mind</strong>. Like it knows it killed the world once and got a taste for murder. <em>After the avalanche, it took us a week to climb out</em>. Now, I don't know exactly when we turned on each other, but I know that seven of us survived the slide... and only five made it out.

Now we took an oath, that I'm breaking now. [We said](#) we'd say it was the snow that killed the other two, but it wasn't.  Nature is lethal but it doesn't hold a candle to man.

Like inline code?  Here is a string for you <code>010101010</code>.

### Lead Body Copy

<p class="lead">Vivamus sagittis lacus vel augue laoreet rutrum faucibus dolor auctor. Duis mollis, est non commodo luctus.</p>


### Headings

All HTML headings, `<h1>` through `<h6>`, are available. `.h1` through `.h6` classes are also available, for when you want to match the font styling of a heading but still want your text to be displayed inline.

<h1>h1. Heading</h1>
<h2>h2. Heading</h2>
<h3>h3. Heading</h3>
<h4>h4. Heading</h4>
<h5>h5. Heading</h5>
<h6>h6. Heading</h6>


<h1>h1. Heading <small>Secondary text</small></h1>
<h2>h2. Heading <small>Secondary text</small></h2>
<h3>h3. Heading <small>Secondary text</small></h3>
<h4>h4. Heading <small>Secondary text</small></h4>
<h5>h5. Heading <small>Secondary text</small></h5>
<h6>h6. Heading <small>Secondary text</small></h6>

### Blockquotes

<blockquote>
  <p>Lorem ipsum dolor sit amet, consectetur adipiscing elit. Integer posuere erat a ante.</p>
  <footer>Someone famous in <cite title="Source Title">Source Title</cite></footer>
</blockquote>


### Lists

* `mkdocs new [dir-name]` - Create a new project.
* `mkdocs serve` - Start the live-reloading docs server.
* `mkdocs build` - Build the documentation site.
* `mkdocs help` - Print this help message.

### Horizontal Description Lists

<dl class="dl-horizontal">
  <dt>Something</dt>
  <dd>This is something</dd>
  <dt>SomethingElse</dt>
  <dd>This is something else</dd>
</dl>

### Contextual Colors

<p class="text-muted">Lorem ipsum dolor sit amet, consectetur adipiscing elit. Integer posuere erat a ante.</p>
<p class="text-primary">Lorem ipsum dolor sit amet, consectetur adipiscing elit. Integer posuere erat a ante.</p>
<p class="text-success">Lorem ipsum dolor sit amet, consectetur adipiscing elit. Integer posuere erat a ante.</p>
<p class="text-info">Lorem ipsum dolor sit amet, consectetur adipiscing elit. Integer posuere erat a ante.</p>
<p class="text-warning">Lorem ipsum dolor sit amet, consectetur adipiscing elit. Integer posuere erat a ante.</p>
<p class="text-danger">Lorem ipsum dolor sit amet, consectetur adipiscing elit. Integer posuere erat a ante.</p>

## Code

### Inline Code

This is an example of inline code `#import requests`

<h3>Preformatted Code Blocks <small>with Syntax Highlighting</small></h3>

<pre><code class="python">def convert_markdown(markdown_source, config, site_navigation=None):
    """
    Convert the Markdown source file to HTML as per the config and
    site_navigation. Return a tuple of the HTML as a string, the parsed table
    of contents, and a dictionary of any metadata that was specified in the
    Markdown file.
    """

    extensions = [
        RelativePathExtension(site_navigation, config['strict'])
    ] + config['markdown_extensions']

    return utils.convert_markdown(
        markdown_source=markdown_source,
        extensions=extensions,
        extension_configs=config['mdx_configs']
    )
</code></pre>


## Tables

### Striped Table

<table class="table table-striped">
  <thead>
	  <tr>
	  	<th>#</th>
	  	<th>Head 1</th>
	  	<th>Head 2</th>
	  	<th>Head 3</th>
	  </tr>
	  </thead>
	  <tbody>
	  <tr>
	  	<th scope="row">1</th>
	  	<td>Box 1</td>
	  	<td>Box 2</td>
	  	<td>Box 3</td>
	  </tr>
	    <tr>
	    <th scope="row">2</th>
	  	<td>Box 4</td>
	  	<td>Box 5</td>
	  	<td>Box 6</td>
	  </tr>
	  <tr>
	    <th scope="row">3</th>
	  	<td>Box 7</td>
	  	<td>Box 8</td>
	  	<td>Box 9</td>
	  </tr>
  </tbody>
</table>

### Bordered Table

<table class="table table-bordered">
  <thead>
	  <tr>
	  	<th>#</th>
	  	<th>Head 1</th>
	  	<th>Head 2</th>
	  	<th>Head 3</th>
	  </tr>
	  </thead>
	  <tbody>
	  <tr>
	  	<th scope="row">1</th>
	  	<td>Box 1</td>
	  	<td>Box 2</td>
	  	<td>Box 3</td>
	  </tr>
	    <tr>
	    <th scope="row">2</th>
	  	<td>Box 4</td>
	  	<td>Box 5</td>
	  	<td>Box 6</td>
	  </tr>
	  <tr>
	    <th scope="row">3</th>
	  	<td>Box 7</td>
	  	<td>Box 8</td>
	  	<td>Box 9</td>
	  </tr>
  </tbody>
</table>

## Buttons

### Default Buttons

<a class="btn btn-default" href="#" role="button">Link</a>
<button class="btn btn-default" type="submit">Button</button>
<input class="btn btn-default" type="button" value="Input">
<input class="btn btn-default" type="submit" value="Submit">

### Styled Buttons

<!-- Standard button -->
<button type="button" class="btn btn-default">Default</button>

<!-- Provides extra visual weight and identifies the primary action in a set of buttons -->
<button type="button" class="btn btn-primary">Primary</button>

<!-- Indicates a successful or positive action -->
<button type="button" class="btn btn-success">Success</button>

<!-- Contextual button for informational alert messages -->
<button type="button" class="btn btn-info">Info</button>

<!-- Indicates caution should be taken with this action -->
<button type="button" class="btn btn-warning">Warning</button>

<!-- Indicates a dangerous or potentially negative action -->
<button type="button" class="btn btn-danger">Danger</button>

### Button Sizes

<p>
  <button type="button" class="btn btn-primary btn-lg">Large button</button>
  <button type="button" class="btn btn-default btn-lg">Large button</button>
</p>
<p>
  <button type="button" class="btn btn-primary">Default button</button>
  <button type="button" class="btn btn-default">Default button</button>
</p>
<p>
  <button type="button" class="btn btn-primary btn-sm">Small button</button>
  <button type="button" class="btn btn-default btn-sm">Small button</button>
</p>
<p>
  <button type="button" class="btn btn-primary btn-xs">Extra small button</button>
  <button type="button" class="btn btn-default btn-xs">Extra small button</button>
</p>

### Block Level Buttons

<button type="button" class="btn btn-primary btn-lg btn-block">Block level button</button>
<button type="button" class="btn btn-default btn-lg btn-block">Block level button</button>


