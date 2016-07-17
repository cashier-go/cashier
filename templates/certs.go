package templates

// Token is the page users see when authenticated.
const Certs = `<html>
  <head>
    <title>Certs</title>
    <style>
      <!--
      body {
        font-family: sans-serif;
        background-color: #edece4;
        margin-top: 120px;
      }
      .code {
        background-color: #26292B;
        border: none;
        color: #fff;
        font-family: monospace;
        font-size: 13;
        font-weight: bold;
        height: auto;
        margin: 12px 12px 12px 12px;
        padding: 24px 12px 12px 12px;
        resize: none;
        text-align: center;
        width: 960px;
      }
      ::selection {
        background: #32d0ff;
        color: #000;
      }
      ::-moz-selection {
        background: #32d0ff;
        color: #000;
      }
      -->
    </style>
  </head>
  <body>

  <form action="/admin/revoke" method="post" id="form_revoke">
  {{ .CSRF }}
  <table>
	<tr>
		<th>ID</th>
		<th>Created</th>
		<th>Expires</th>
		<th>Principals</th>
		<th>Revoked</th>
		<th>Revoke</th>
	</tr>

  {{range .Certs}}
	<tr>
	<td>{{.KeyID}}</td>
	<td>{{.CreatedAt}}</td>
	<td>{{.Expires}}</td>
	<td>{{.Principals}}</td>
	<td>{{.Revoked}}</td>
	<td>
		{{if not .Revoked}}
		<input type="checkbox" value="{{.KeyID}}" name="cert_id" id="cert_id" />
		{{end}}
	</td>
	</tr>
  {{ end }}
  </table>
  </form>
  <button type="submit" form="form_revoke" value="Submit">Submit</button>
  </body>
</html>`
