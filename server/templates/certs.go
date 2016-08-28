package templates

// Certs lists all unexpired issued certificates.
const Certs = `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>Issued Certificates</title>

	<link rel="stylesheet" href="/static/css/normalize.css">
	<link rel="stylesheet" href="/static/css/skeleton.css">
	<link href="https://fonts.googleapis.com/css?family=Source+Sans+Pro" rel="stylesheet">
</head>
<body>
	<div class="container">
		<div class="page-header">
			<h2>Issued SSH Certificates</h2>
		</div>

		<div id="issued">
			<input class="u-full-width search" type="text" placeholder="Search" id="q" />
			<form action="/admin/revoke" method="post" id="form_revoke">
			{{ .CSRF }}
			<table id="cert-table">
				<thead>
				<tr>
					<th>ID</th>
					<th>Created</th>
					<th>Expires</th>
					<th>Principals</th>
					<th>Revoked</th>
					<th>Revoke</th>
				</tr>
				</thead>
				<tbody class="list">
				{{range .Certs}}
					<tr>
					<td class="keyid">{{.KeyID}}</td>
					<td>{{.CreatedAt}}</td>
					<td>{{.Expires}}</td>
					<td class="principals">{{.Principals}}</td>
					<td>{{.Revoked}}</td>
					<td>{{if not .Revoked}}<input style="margin:0;" type="checkbox" value="{{.KeyID}}" name="cert_id" id="cert_id" />{{end}}</td>
					</tr>
				{{ end }}
				</tbody>
			</table>
			</form>
			<button class="button-primary" type="submit" form="form_revoke" value="Submit">Submit</button>
		</div>
	</div>
</body>
<script src="/static/js/list.min.js"></script>
<script>
var options = {
	valueNames: [ 'keyid', 'principals' ],
}
var issuedList = new List('issued', options);
</script>
</html>
`
