package experiment

import (
	"html/template"
	"io"
)

type Results []Result

type Result struct {
	MessageRate      uint
	MessagesSent     uint
	MessagesReceived uint
	LossPercentage   float64
}

func (r Results) Render(w io.Writer) {
	view.Execute(w, r)
}

var view = template.Must(template.New("results.html").Parse(
	`
<!doctype html>
<head>
	<title>metronbenchmark Results</title>
</head>
<body>
<h1>metronbenchmark Results</h1>
{{with .}}
<table>
	<thead>
	<tr>
		<th>Message Rate</th>
		<th>Messages Sent</th>
		<th>Messages Received</th>
		<th>Message Loss Percentage</th>
	</tr>
	</thead>
	<tbody>
	{{range .}}
	<tr>
		<th>{{.MessageRate}}</th>
		<td>{{.MessagesSent}}</td>
		<td>{{.MessagesReceived}}</td>
		<td>{{printf "%3.2f" .LossPercentage}}%</td>
	</tr>
	{{end}}
	</tbody>
</table>
{{else}}
<p>There are no results. Please issue a request to the /start endpoint.</p>
{{end}}
</body>
`,
))
