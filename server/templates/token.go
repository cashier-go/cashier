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
	body {
		font-family: 'Source Sans Pro', sans-serif;
	}
	.token-display {
		background-color: #eee;
		border: solid 1px #ccc;
		font-family: 'Source Code Pro', monospace;
		font-weight: bold;
		height: 200px;
		margin: 12px 12px 12px 12px;
		padding: 24px 12px 12px 12px;
		resize: none;
		text-align: center;
	}
	.error {
		color:#000!important;
		background-color:#ffdddd!important;
		border: solid 1px #ccc;
		font-size: 16pt;
		margin: 12px 12px 12px 12px;
		padding: 24px 12px 12px 12px;
	}
	.success {
		color:#000!important;
		background-color:#ddffdd!important;
		border: solid 1px #ccc;
		font-size: 16pt;
		margin: 12px 12px 12px 12px;
		padding: 24px 12px 12px 12px;
	}
	-->
	</style>
</head>
<body>
	<div class="container">
		<div class="page-header">
			<h2>Access Token</h2>
		</div>
		<div id="auto-token-status">
		</div>
		<div>
			<textarea style="font-size: 12pt" class="u-full-width token-display" readonly spellcheck="false" onclick="this.focus();this.select();">{{.Token}}.</textarea>
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
	{{ if .Localserver }}
	<script>
		var xhr = new XMLHttpRequest();
		var output = document.getElementById("auto-token-status");
		xhr.onload = function() {
			if (xhr.status == 200 && xhr.response == "ok") {
				output.classList.add("success");
				output.innerHTML = "Authentication complete - you can close this window";
			} else {
				output.classList.add("error");
				output.innerHTML = "Failed to automatically add credentials.<br />" +
				"Copy the code below and paste it into your terminal";
			}
		};
		xhr.onerror = function() {
				output.classList.add("error");
				output.innerHTML = "Failed to automatically add credentials.<br />" +
				"Copy the code below and paste it into your terminal";
		};
		xhr.open("GET", {{ printf "http://localhost:%s?token=%s" .Localserver .Token }});
		xhr.send();
	</script>
	{{ end }}
</body>
</html>
`
