package report_test

import (
	"fmt"

	"github.com/kaiser-chris/pdx-parser-go/report"
)

func ExampleDiagnostics_Filter() {
	var collector report.Collector

	collector.Addf(report.SeverityInfo, "My Mod", "common/goods/zz.txt", 3, 1, "good tin", "unknown field shiny, kept but not interpreted")
	collector.Addf(report.SeverityWarning, "My Mod", "common/goods/zz.txt", 4, 9, "good tin", "cost should be a number, found cheap; ignored")

	for _, diagnostic := range collector.Diagnostics.Filter(report.SeverityWarning) {
		fmt.Println(diagnostic)
	}

	// Output:
	// warning: common/goods/zz.txt:4:9: good tin: cost should be a number, found cheap; ignored
}
