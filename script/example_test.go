package script_test

import (
	"fmt"

	"github.com/kaiser-chris/pdx-parser-go/script"
)

func ExampleParse() {
	document := script.Parse(`
steel = {
	cost = 50
	category = industrial
	texture = "gfx/interface/icons/goods_icons/steel.dds"
	possible = { level >= 5 }
	color = hsv { 0.1 0.2 0.3 }
}
`)

	steel, _ := document.Get("steel")

	cost, _ := steel.Get("cost")
	category, _ := steel.Get("category")
	texture, _ := steel.Get("texture")
	fmt.Println(cost.Number, category.Text, texture.Text, texture.Quoted)

	possible, _ := steel.Get("possible")
	level := possible.Fields[0]
	fmt.Println(level.Key, level.Operator, level.Value.Number)

	color, _ := steel.Get("color")
	channels, _ := color.Value.Numbers()
	fmt.Println(color.Tag, channels)

	// Output:
	// 50 industrial gfx/interface/icons/goods_icons/steel.dds true
	// level >= 5
	// hsv [0.1 0.2 0.3]
}

// A broken file still yields every definition around the damage, and says
// what it skipped.
func ExampleParse_recovery() {
	document := script.Parse(`
iron = { cost = 40 } }
coal = {
	cost =
	category = industrial
}
steel = { cost = 50
`)

	for _, field := range document.Fields {
		fmt.Println(field.Key)
	}

	for _, warning := range document.Warnings {
		fmt.Println(warning)
	}

	// Output:
	// iron
	// coal
	// steel
	// line 2 column 22: ignored a closing brace that closes nothing
	// line 4 column 7: "cost" has no value after its =
	// line 7 column 9: block is never closed, closing it at the end of the file
}

func ExampleParse_variables() {
	document := script.Parse(`
@base = 40
@markup = 0.25
price = @[ base * ( 1 + markup ) ]
`)

	price, _ := document.Get("price")
	fmt.Println(price.Number)

	// Output:
	// 50
}

// Keys repeat. Get returns the last occurrence, as the games read a repeated
// single value; All returns every occurrence, for keys meant to repeat.
func ExampleNode_All() {
	document := script.Parse(`
popneed_heating = {
	entry = { goods = coal }
	entry = { goods = electricity }
}
`)

	need, _ := document.Get("popneed_heating")

	for _, entry := range need.All("entry") {
		goods, _ := entry.Get("goods")
		fmt.Println(goods.Text)
	}

	// Output:
	// coal
	// electricity
}

// Every field knows where it is in the input, so a tool can replace one
// definition without touching the rest of the file.
func ExampleField_spans() {
	input := "iron = { cost = 40 } # the ore\nsteel = { cost = 50 }\n"
	document := script.Parse(input)

	steel := document.Fields[1]
	edited := input[:steel.Start] + "steel = { cost = 55 }" + input[steel.End:]

	fmt.Print(edited)

	// Output:
	// iron = { cost = 40 } # the ore
	// steel = { cost = 55 }
}
