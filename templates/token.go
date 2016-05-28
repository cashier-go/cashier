package templates

const Token = `<html>
  <head>
    <title>YOUR TOKEN!</title>
    <style>
      <!--
      body {
        text-align: center;
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
    <h2>
      This is your token. There are many like it but this one is yours.
    </h2>
    <textarea class="code" readonly spellcheck="false" onclick="this.focus();this.select();">{{.Token}}</textarea>
    <h2>
      The token will expire in &lt; 1 hour.
    </h2>
  </body>
</html>`
