package templates

// Certs lists all unexpired issued certificates.
const Certs = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <!-- The above 3 meta tags *must* come first in the head; any other head content must come *after* these tags -->
    <title>Issued Certificates</title>

    <!-- Bootstrap -->
    <link href="/static/css/bootstrap.min.css" rel="stylesheet">

    <!-- HTML5 shim and Respond.js for IE8 support of HTML5 elements and media queries -->
    <!-- WARNING: Respond.js doesn't work if you view the page via file:// -->
    <!--[if lt IE 9]>
      <script src="https://oss.maxcdn.com/html5shiv/3.7.3/html5shiv.min.js"></script>
      <script src="https://oss.maxcdn.com/respond/1.4.2/respond.min.js"></script>
    <![endif]-->
  </head>
  <body>
  <div class="container">
  <div class="page-header">
  <h1>Issued SSH Certificates</h1>
  </div>

  <form action="/admin/revoke" method="post" id="form_revoke">
  {{ .CSRF }}
  <table class="table table-hover table-condensed">
	<tr>
		<th>ID</th>
		<th>Created</th>
		<th>Expires</th>
		<th>Principals</th>
		<th>Revoked</th>
		<th>Revoke</th>
	</tr>

  {{range .Certs}}
	<div class="checkbox">
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
	</div>
  {{ end }}
  </table>
  </form>
  <button class="btn btn-primary" type="submit" form="form_revoke" value="Submit">Submit</button>
  </div>

    <!-- jQuery (necessary for Bootstrap's JavaScript plugins) -->
    <script src="https://ajax.googleapis.com/ajax/libs/jquery/1.12.4/jquery.min.js"></script>
    <!-- Include all compiled plugins (below), or include individual files as needed -->
    <script src="/static/js/bootstrap.min.js"></script>
  </body>
</html>

`
