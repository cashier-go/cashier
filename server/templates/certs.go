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
<body onload="loadCerts()">
	<div class="container">
		<div class="page-header">
			<h2>Issued SSH Certificates</h2>
		</div>

		<div id="issued">
			<input class="u-full-width search" type="text" placeholder="Search" id="q" />
			<button class="button-primary" id="toggle-certs" onclick="toggleExpired()">Show Expired</button>
			<form action="/admin/revoke" method="post" id="form_revoke">
			{{ .csrfField }}
			<table id="cert-table">
				<thead>
				<tr>
					<th>ID</th>
					<th>Created</th>
					<th>Expires</th>
					<th>Principals</th>
					<th>Message</th>
					<th>Revoked</th>
					<th>Revoke</th>
				</tr>
				</thead>
				<tbody id="list" class="list">
				<tr>
					<td class="keyid"></td>
					<td></td>
					<td></td>
					<td class="principals"></td>
					<td></td>
					<td></td>
					<td></td>
					</tr>
				</tbody>
			</table>
			</form>
			<button class="button-primary" type="submit" form="form_revoke" value="Revoke">Revoke</button>
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
<script src="/static/js/table.js"></script>
</html>
`
