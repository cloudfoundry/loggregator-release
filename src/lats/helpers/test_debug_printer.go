package helpers

type TestDebugPrinter struct {
	dump string
}

func (printer *TestDebugPrinter) Print(title, dump string) {
	printer.dump = dump
}

func (printer *TestDebugPrinter) Dump() string {
	return printer.dump
}
