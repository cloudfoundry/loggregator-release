package experiment_test

import (
	"bytes"
	"regexp"
	"strings"
	"tools/metronbenchmark/experiment"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Results", func() {
	It("warns if there are no results and does not include a table", func() {
		results := []experiment.Result{}
		expectedHTML := compactHTML(
			`<!doctype html>
			 <head>
			 	<title>metronbenchmark Results</title>
			 </head>
			 <body>
			 <h1>metronbenchmark Results</h1>
			 <p>There are no results. Please issue a request to the /start endpoint.</p>
			 </body>`)

		Expect(asHTML(results)).To(Equal(expectedHTML))
	})

	It("adds a new result", func() {
		results := []experiment.Result{
			{
				MessageRate:      70,
				MessagesSent:     20,
				MessagesReceived: 10,
				LossPercentage:   50,
			},
		}

		expectedHTML := compactHTML(
			`<!doctype html>
			 <head>
			 	<title>metronbenchmark Results</title>
			 </head>
			 <body>
			 <h1>metronbenchmark Results</h1>
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

			 	<tr>
			 		<th>70</th>
			 		<td>20</td>
			 		<td>10</td>
			 		<td>50.00%</td>
			 	</tr>

			 	</tbody>
			 </table>
			 </body>`)

		actual := asHTML(results)
		Expect(actual).To(Equal(expectedHTML))
		Expect(actual).NotTo(ContainSubstring(`There are no results.`))
	})

	It("adds multiple results", func() {
		results := []experiment.Result{
			{
				MessageRate:      100,
				MessagesSent:     100,
				MessagesReceived: 75,
				LossPercentage:   25,
			},
			{
				MessageRate:      200,
				MessagesSent:     180,
				MessagesReceived: 60,
				LossPercentage:   100 * float64(120) / 180,
			},
		}

		expectedHTML := compactHTML(
			`<tr>
				<th>100</th>
				<td>100</td>
				<td>75</td>
				<td>25.00%</td>
			</tr>
			<tr>
				<th>200</th>
				<td>180</td>
				<td>60</td>
				<td>66.67%</td>
			</tr>`)

		Expect(asHTML(results)).To(ContainSubstring(expectedHTML))
	})
})

func compactHTML(formattedHTML string) string {
	result := strings.Replace(formattedHTML, "\n", "", -1)
	re := regexp.MustCompile(">\\s+<")
	result = re.ReplaceAllString(result, "><")

	return result
}

func asHTML(r experiment.Results) string {
	buffer := bytes.Buffer{}

	r.Render(&buffer)
	return compactHTML(string(buffer.Bytes()))
}
