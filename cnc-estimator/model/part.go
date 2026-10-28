package model

type Stock struct {
	Width  float64
	Length float64
	Height float64
}

type Part struct {
	Material   string
	Stock      Stock
	SourceFile string
	Features   []Feature
}
