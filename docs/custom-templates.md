# Custom Templates

To customize the HTML pages rendered by Gangplank, you may provide a set of custom templates to use instead of the built-in ones.

:exclamation: **Important: The data passed to the templates might change between versions, and we do not guarantee that we will maintain backwards compatibility. If using custom templates, extra care must be taken when upgrading Gangplank.**

To enable this feature, set the `customHTMLTemplatesDir` option in Gangplank's configuration file to a directory that contains the following custom templates:

- home.tmpl: Home page template.
- commandline.tmpl: Post-login template that typically lists the commands needed to configure `kubectl`.

The templates are processed using Go's `html/template` [package][0].

## Custom Static Assets

To customize the static assets (such as CSS files) served by Gangplank, set the `customStaticDir` option in Gangplank's
configuration file to a directory containing your custom static files (e.g. `style.css`).

[0]: https://golang.org/pkg/html/template/
