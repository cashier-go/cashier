package templates

// Retry is the page users see when an authentication attempt could not be
// completed, most often because the token page was reloaded and its one-time
// code had already been used.
const Retry = `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>Sign in again</title>

	<link rel="stylesheet" href="/static/css/normalize.css">
	<link rel="stylesheet" href="/static/css/skeleton.css">
	<link href="https://fonts.googleapis.com/css?family=Source+Sans+Pro" rel="stylesheet">
	<style>
	<!--
	body {
		font-family: 'Source Sans Pro', sans-serif;
	}
	-->
	</style>
</head>
<body>
	<div class="container">
		<h4>That sign-in link has already been used</h4>
		<p>
			Tokens are issued once and are not stored, so this page cannot be
			reloaded. <a href="/auth/login">Start a new sign-in</a> to get another one.
		</p>
	</div>
</body>
</html>
`
