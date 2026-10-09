module github.com/MickMake/GoTradie

go 1.25

require (
	github.com/MickMake/GoBunnings v0.0.0
	github.com/MickMake/GoInvoiceNinja v0.0.0
	go.yaml.in/yaml/v3 v3.0.5
)

replace github.com/MickMake/GoBunnings => ../GoBunnings

replace github.com/MickMake/GoInvoiceNinja => ../GoInvoiceNinja
