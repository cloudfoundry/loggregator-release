package linter

import (
	"bufio"
	"fmt"
	"os"
)

const (
	blue  string = "\033[34m"
	red   string = "\033[31m"
	reset string = "\033[0m"
)

func PrintProblem(filename string, p Problem) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(os.Stdout, "%s%s%s\n", red, p.Kind, reset)
	fmt.Fprintf(os.Stdout, "%s:%d:%d\n", filename, p.Position.Line, p.Position.Column)

	buf := bufio.NewScanner(file)
	l := p.Position.Line
	for i := 1; i < l-5; i++ {
		if !buf.Scan() {
			return nil
		}
	}

	s := l - 5
	if s < 1 {
		s = 1
	}

	for i := s; i <= l+5; i++ {
		if !buf.Scan() {
			return nil
		}

		var (
			prefix    string
			colorLine bool
		)
		color := blue
		prefix = "  "
		if i == l {
			color = red
			colorLine = true
			prefix = "=>"
		}

		prefix = fmt.Sprintf("%s%4d:\t", prefix, i)
		println(prefix, buf.Text(), color, colorLine)
	}
	fmt.Println()
	return nil
}

func println(prefix, str, color string, colorLine bool) {
	prefix = fmt.Sprintf("%s%s%s", color, prefix, reset)
	if colorLine {
		fmt.Fprintf(os.Stdout, "%s%s%s%s\n", prefix, color, str, reset)
		return
	}
	fmt.Fprintf(os.Stdout, "%s%s\n", prefix, str)
}
