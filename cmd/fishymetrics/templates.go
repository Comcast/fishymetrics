/*
 * Copyright 2024 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

type indexAppData struct {
	Hostname string
}

const postPayload string = "`{\"host\": \"${host}\"}`"

const indexTmpl string = `<html>
  <head>
    <title>Fishymetrics Exporter</title>
    <style>
      .links, .build-info {
        display: flex;
      }
      h3, p {
        padding-right: 1em;
      }
      label {
        display: inline-block;
        width: 75px;
      }
      form label {
        margin: 10px;
      }
      form input {
        margin: 10px;
      }
    </style>
  </head>
  <body>
    <h1>Fishymetrics Exporter</h1>
    <div class="build-info">
      <p><b>build date:</b> {{ .Date }}</p>
      <p><b>revision:</b> {{ .GitRevision }}</p>
      <p><b>version:</b> {{ .GitVersion }}</p>
    </div>
    <div class="links">
      <h3><a href="ignored">Ignored Hosts</a></h3>
      <h3><a href="metrics">Metrics</a></h3>
    </div>
    <form action="scrape">
      <label>Target:</label> <input type="text" name="target" placeholder="ip or fdqn"><br>
      <label>Model:</label> <input type="text" name="model" placeholder="chassis model i.e. dl360"><br>
      <input type="submit" value="Submit">
    </form>
  </body>
</html>
`

const ignoredTmpl string = `<html>
<head>
  <title>Fishymetrics Exporter</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta http-equiv="refresh" content="60">
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@4.6.1/dist/css/bootstrap.min.css">
  <script src="https://cdn.jsdelivr.net/npm/jquery@3.6.0/dist/jquery.slim.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/popper.js@1.16.1/dist/umd/popper.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/bootstrap@4.6.1/dist/js/bootstrap.bundle.min.js"></script>
  <script src="https://ajax.googleapis.com/ajax/libs/jquery/3.6.0/jquery.min.js"></script>
  <style>
    .spinner-border {
      width: 1.5rem;
      height: 1.5rem;
    }
    .error-text {
      color: red;
      font-style: oblique;
    }
    h1 {
      padding: 1rem;
    }
    h3 {
      padding-left: 1rem;
    }
  </style>
</head>
<body>
  <h1>Ignored Hosts</h1>
  <h3><a href="../">Home</a></h3>
  <div>
    <ul>
      {{range .}}
      <li>{{.Name}}
        <button type="button" onclick="testConn('{{ .Name }}')">Test</button>
        <button type="button" onclick="remove('{{ .Name }}')">Remove</button>
        <div style="display: inline" id="{{ .Name }}-result" hidden></div>
        <div id="{{ .Name }}-spinner" class="spinner-border" hidden></div>
        <div style="display: inline" id="{{ .Name }}-error" class="error-text" hidden></div>
      </li>
      {{end}}
    </ul>
  </div>
<script>
  function testConn(host) {
    const icon = document.getElementById(host+"-result")
    const errorText = document.getElementById(host+"-error")
    // show spinner
    const spinner = document.getElementById(host+"-spinner")
    spinner.hidden = false;

    $.post("ignored/test-conn", ` + postPayload + `, (data, status) => {
      const resp = JSON.parse(data)
      if (resp.connectionTest === true) {
        spinner.hidden = true;
        icon.hidden = false;
        icon.innerHTML = "&#9989;"; // green check
      } else {
        spinner.hidden = true;
        icon.hidden = false;
        icon.innerHTML = "&#10060;"; // red X
        errorText.hidden = false;
        errorText.innerHTML = resp.error;
      }
    }).fail((data) => {
      spinner.hidden = true;
      icon.hidden = false;
      icon.innerHTML = "&#10060;"; // red X
      const resp = JSON.parse(data.responseText)
      errorText.hidden = false;
      errorText.innerHTML = resp.error;
    });
  }

  function remove(host) {
    const errorText = document.getElementById(host+"-error")

    $.post("ignored/remove", ` + postPayload + `, (data, status) => {
      if (status === "success") {
        location.reload();
      }
    }).fail((data) => {
      const resp = JSON.parse(data.responseText)
      errorText.hidden = false;
      errorText.innerHTML = resp.error;
    });
  }
</script>
</body>
</html>
`
