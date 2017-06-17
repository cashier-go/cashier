package templates

// Token is the page users see when authenticated.
const Token = `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>Token</title>

	<link rel="stylesheet" href="/static/css/normalize.css">
	<link rel="stylesheet" href="/static/css/skeleton.css">
	<link href="https://fonts.googleapis.com/css?family=Source+Sans+Pro" rel="stylesheet">
	<link href="https://fonts.googleapis.com/css?family=Source+Code+Pro" rel="stylesheet">
	<style>
	<!--
	.code {
		background-color: #eee;
		border: solid 1px #ccc;
		font-family: 'Source Code Pro', monospace;
		font-weight: bold;
		height: 120px;
		margin: 12px 12px 12px 12px;
		padding: 24px 12px 12px 12px;
		resize: none;
		text-align: center;
	}
	-->
	</style>
</head>
<body>
	<div class="container">
		<div class="page-header">
			<h2>Access Token</h2>
		</div>
		<div>
			<textarea style="font-size: 12pt" class="u-full-width code" readonly spellcheck="false" onclick="this.focus();this.select();">{{.Token}}</textarea>
			<h3>
				The token will expire in &lt; 1 hour.
			</h3>
		</div>
		<div>
			<h4>
				<a href="/admin/certs">Previously Issued Certificates</a>
			</h4>
		</div>
	</div>
</body>
</html>
`
