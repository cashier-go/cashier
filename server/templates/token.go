package templates

// Token is the page users see when authenticated.
const Token = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <!-- The above 3 meta tags *must* come first in the head; any other head content must come *after* these tags -->
    <title>Token</title>

    <!-- Bootstrap -->
    <link href="/static/css/bootstrap.min.css" rel="stylesheet">

    <!-- HTML5 shim and Respond.js for IE8 support of HTML5 elements and media queries -->
    <!-- WARNING: Respond.js doesn't work if you view the page via file:// -->
    <!--[if lt IE 9]>
      <script src="https://oss.maxcdn.com/html5shiv/3.7.3/html5shiv.min.js"></script>
      <script src="https://oss.maxcdn.com/respond/1.4.2/respond.min.js"></script>
    <![endif]-->
    <style>
      <!--
      .code {
        border: none;
        font-family: monospace;
        font-weight: bold;
        height: auto;
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
  <h1>Access Token</h1>
  </div>
	<div>
	<textarea style="font-size: 15pt" class="form-control code" readonly spellcheck="false" onclick="this.focus();this.select();">{{.Token}}</textarea>
    <h2>
      The token will expire in &lt; 1 hour.
    </h2>
	</div>
  </div>
    <!-- jQuery (necessary for Bootstrap's JavaScript plugins) -->
    <script src="https://ajax.googleapis.com/ajax/libs/jquery/1.12.4/jquery.min.js"></script>
    <!-- Include all compiled plugins (below), or include individual files as needed -->
    <script src="/static/js/bootstrap.min.js"></script>
  </body>
</html>
`
