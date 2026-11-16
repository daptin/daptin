package conformation

import "testing"

func TestDeclaredTransforms(t *testing.T) {
	for _, tc := range []struct {
		tags, input, want string
	}{
		{"lower,snake", "OAuth.Login.Response", "oauth_login_response"},
		{"lower,snake", "x+y", "x_y"},
		{"lower,snake", "MailAccountId", "mailaccountid"},
		{"camel", "_compose!!draft 2fa_", "composeDraft2fa"},
		{"camel", "mañana", "maAna"},
		{"snake", "XMLHTTPRequest", "xml_http_request"},
		{"snake", "fooBAR", "foo_b_a_r"},
		{"slug", "mail.smtp host", "mail-smtp-host"},
		{"trim,email", "  Sender@EXAMPLE.COM\u00a0", "Sender@example.com"},
		{"email", "Some@Address@EXAMPLE.COM", "Some@Address@example.com"},
		{"trim,title", " \tmail box\n", "Mail Box"},
		{"ucfirst", "élève", "Élève"},
		{"name", "  élise  -  marie o''neil 2", "Élise-Marie O'Neil"},
		{"num", "m2—３f0", "20"},
		{"!num", "m2—３f0", "m—３f"},
		{"alpha,upper", "m2—éf0", "MÉF"},
		{"!alpha", "m2—éf0", "2—0"},
		{"!html", "<mail>&", "&lt;mail&gt;&amp;"},
		{"!js", "<mail>", "\\u003Cmail\\u003E"},
		{"ltrim,rtrim", "  \tmail  ", "\tmail"},
		{"trim,unregistered,upper", " mail ", "MAIL"},
	} {
		t.Run(tc.tags+"/"+tc.input, func(t *testing.T) {
			if got := TransformString(tc.input, tc.tags); got != tc.want {
				t.Errorf("TransformString(%q, %q) = %q; want %q", tc.input, tc.tags, got, tc.want)
			}
		})
	}
}
