package web

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"smetchic/application"
	"smetchic/model"
)

type costFormula struct{ Formula, Values string }
type costExplanation struct {
	Batch    []costFormula
	PerPiece costFormula
	Note     string
}

type costTotalDisplay struct {
	Name   string
	Amount float64
	costFormula
}

func savedCostTotals(b *model.ProductionCostBreakdown) []costTotalDisplay {
	money := func(value float64) string { return formatMoney(value, b.Currency) }
	sum := b.MaterialCost + b.MachineCost + b.OperatorCost + b.ProgrammingCost + b.ToolingCost + b.SetupCost + b.RentCost + b.UtilitiesCost + b.OtherOverheadCost
	totals := []costTotalDisplay{
		{Name: "Себестоимость партии", Amount: b.TotalCost},
		{Name: "Себестоимость 1 детали", Amount: b.TotalPerPieceCost},
		{Name: "Прибыль", Amount: b.Profit},
		{Name: "Цена партии", Amount: b.SellingPrice},
		{Name: "Цена 1 детали", Amount: b.SellingPricePerPiece},
	}
	set := func(index int, formula, values string, calculated float64) {
		if !math.IsNaN(calculated) && !math.IsInf(calculated, 0) && math.Abs(calculated-totals[index].Amount) <= 0.005 {
			totals[index].costFormula = costFormula{Formula: formula, Values: values + " = " + money(totals[index].Amount)}
		}
	}
	var terms []string
	for _, amount := range []float64{b.MaterialCost, b.MachineCost, b.OperatorCost, b.ProgrammingCost, b.ToolingCost, b.SetupCost, b.RentCost, b.UtilitiesCost, b.OtherOverheadCost} {
		terms = append(terms, money(amount))
	}
	set(0, "Материал + затраты на станок + оператор + программирование + инструмент + наладка + аренда + коммунальные + прочие накладные", strings.Join(terms, " + "), sum)
	if b.Quantity > 0 {
		set(1, "Себестоимость партии ÷ количество деталей", money(b.TotalCost)+" ÷ "+costNumber(float64(b.Quantity)), b.TotalCost/float64(b.Quantity))
		set(4, "Цена партии ÷ количество деталей", money(b.SellingPrice)+" ÷ "+costNumber(float64(b.Quantity)), b.SellingPrice/float64(b.Quantity))
	}
	set(2, "Себестоимость партии × наценка ÷ 100", money(b.TotalCost)+" × "+costNumber(b.MarkupPercent)+" ÷ 100", b.TotalCost*b.MarkupPercent/100)
	set(3, "Себестоимость партии + прибыль", money(b.TotalCost)+" + "+money(b.Profit), b.TotalCost+b.Profit)
	return totals
}

type savedCostSettings struct {
	CompanyEconomics   *model.CompanyEconomics
	EquipmentEconomics *model.EquipmentEconomics
	Equipment          *model.Equipment
	PersonnelRates     []model.PersonnelRate
	Overheads          []model.CompanyOverhead
	Consumable         *model.Consumable
	Material           *model.Material
	GlobalMaterial     *model.GlobalMaterial
	StockBlank         *model.StockBlank
}

// Settings models historically used Go property names. Accept their snake_case
// equivalents too. String values, including names, are never transformed.
func decodeSavedCostSettings(data []byte, target any) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	var compact func(any) any
	compact = func(value any) any {
		switch item := value.(type) {
		case map[string]any:
			result := make(map[string]any, len(item))
			for key, child := range item {
				result[strings.ReplaceAll(strings.ToLower(key), "_", "")] = compact(child)
			}
			return result
		case []any:
			for i, child := range item {
				item[i] = compact(child)
			}
		}
		return value
	}
	encoded, err := json.Marshal(compact(value))
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, target)
}

func costNumber(value float64) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 6, 64), "0"), ".")
}

func explainSavedCosts(page *lifecyclePage, settingsData, inputData []byte) map[string]costExplanation {
	b := page.Result.CostBreakdown
	out := make(map[string]costExplanation, len(page.CostItems))
	if b == nil {
		return out
	}
	q := float64(b.Quantity)
	money := func(value float64) string { return formatMoney(value, b.Currency) }
	for _, item := range page.CostItems {
		explanation := costExplanation{Note: "Полную формулу по данным этой версии восстановить не удалось. Показаны сохранённые суммы."}
		if q > 0 {
			if math.Abs(item.BatchCost/q-item.PerPieceCost) <= 0.005 {
				explanation.PerPiece = costFormula{
					Formula: "Стоимость статьи на партию ÷ количество деталей = стоимость на 1 деталь",
					Values:  money(item.BatchCost) + " ÷ " + costNumber(q) + " = " + money(item.PerPieceCost),
				}
			} else {
				explanation.PerPiece = costFormula{Formula: "Сохранённая стоимость на 1 деталь", Values: money(item.PerPieceCost)}
			}
		}
		out[item.Name] = explanation
	}
	add := func(name, formula, expression string, calculated float64) {
		explanation, exists := out[name]
		if !exists || math.IsNaN(calculated) || math.IsInf(calculated, 0) {
			return
		}
		var saved float64
		for _, item := range page.CostItems {
			if item.Name == name {
				saved = item.BatchCost
				break
			}
		}
		if math.Abs(calculated-saved) > 0.005 {
			return
		}
		explanation.Batch = []costFormula{{Formula: formula, Values: expression + " = " + money(saved)}}
		explanation.Note = ""
		out[name] = explanation
	}
	if q <= 0 {
		return out
	}
	// This identity uses saved amounts and remains available for sparse old snapshots.
	add("Материал", "Сохранённая стоимость материала на деталь × количество деталей = материал на партию",
		money(b.MaterialCost/q)+" × "+costNumber(q), b.MaterialCost)

	var settings savedCostSettings
	_ = decodeSavedCostSettings(settingsData, &settings)
	meta, err := decodeEstimateVersionSnapshot(inputData)
	if err != nil {
		return out
	}
	input := meta.WorkingInput
	// Some early snapshots kept calculation options without a working document.
	var legacy struct {
		Options  struct{ ToolUsageMinutes, SetupTimeMinutes, ProgrammingHours float64 }
		Revision struct{ ParsedPart model.Part }
	}
	if decodeSavedCostSettings(inputData, &legacy) == nil && input.Name == "" && input.ToolUsageMinutes == 0 && input.SetupTimeMinutes == 0 && input.ProgrammingHours == 0 {
		input.ToolUsageMinutes = legacy.Options.ToolUsageMinutes
		input.SetupTimeMinutes = legacy.Options.SetupTimeMinutes
		input.ProgrammingHours = legacy.Options.ProgrammingHours
	}
	hours := meta.UsedMachineTimeHours
	var root map[string]json.RawMessage
	normalized, _ := normalizeJSONPropertyNames(inputData)
	_ = json.Unmarshal(normalized, &root)
	timeKnown := len(root["used_machine_time_hours"]) > 0
	if !timeKnown && page.Version.ID != "" {
		hours = page.Version.MachineTimeHours / q
		timeKnown = true
	}
	timeExpression := costNumber(hours) + " ч × " + costNumber(q)
	if timeKnown && b.EquipmentEconomicsApplied && settings.EquipmentEconomics != nil {
		e := settings.EquipmentEconomics
		if e.UsefulLifeYears > 0 && e.AnnualAvailableHours > 0 {
			add("Амортизация станка", "(Цена приобретения − остаточная стоимость) ÷ (срок службы, лет × доступные часы в год) × часы на деталь × количество",
				fmt.Sprintf("(%s − %s) ÷ (%s × %s ч) × %s", money(e.PurchasePrice), money(e.ResidualValue), costNumber(e.UsefulLifeYears), costNumber(e.AnnualAvailableHours), timeExpression),
				(e.PurchasePrice-e.ResidualValue)/(e.UsefulLifeYears*e.AnnualAvailableHours)*hours*q)
			for _, annual := range []struct {
				name   string
				amount float64
			}{
				{"Обслуживание станка", e.AnnualMaintenanceCost}, {"Ремонт станка", e.AnnualRepairCost}, {"Прочие расходы станка", e.AnnualOtherCost},
			} {
				add(annual.name, "Годовые расходы ÷ доступные часы в год × часы на деталь × количество",
					money(annual.amount)+" ÷ "+costNumber(e.AnnualAvailableHours)+" ч × "+timeExpression, annual.amount/e.AnnualAvailableHours*hours*q)
			}
		}
		add("Электроэнергия станка", "Потребление, кВт × тариф за кВт·ч × часы на деталь × количество",
			costNumber(e.ConsumptionKW)+" кВт × "+money(e.ElectricityPricePerKWh)+"/кВт·ч × "+timeExpression,
			e.ConsumptionKW*e.ElectricityPricePerKWh*hours*q)
	} else if timeKnown && !b.EquipmentEconomicsApplied {
		rate, known := page.Version.HourlyRate, page.Version.ID != ""
		if settings.Equipment != nil {
			rate, known = settings.Equipment.MachineRatePerHour, true
		}
		if known {
			add("Затраты на станок", "Почасовая ставка станка × часы на деталь × количество", money(rate)+"/ч × "+timeExpression, rate*hours*q)
		}
	}
	var fixed model.ProjectLabor
	if input.ProjectLabor != nil {
		fixed = *input.ProjectLabor
	}
	for _, labor := range []struct {
		name, role          string
		fee                 *float64
		duration            float64
		known               bool
		expression, formula string
	}{
		{"Оператор", "operator", fixed.Operator, hours * q, timeKnown, timeExpression, "часы обработки одной детали × количество"},
		{"Наладка", "setup", fixed.Setup, input.SetupTimeMinutes / 60, len(inputData) > 0, costNumber(input.SetupTimeMinutes) + " мин ÷ 60", "минуты наладки на партию ÷ 60"},
		{"Программирование", "programmer", fixed.Programming, input.ProgrammingHours, len(inputData) > 0, costNumber(input.ProgrammingHours) + " ч", "часы программирования на партию"},
	} {
		module := map[string]string{"operator": application.ModuleOperator, "setup": application.ModuleSetup, "programmer": application.ModuleProgramming}[labor.role]
		if input.Modules != nil && !input.Modules.Has(module) {
			add(labor.name, "Модуль этой работы выключен. Расходы не включены", "0", 0)
			continue
		}
		if labor.fee != nil {
			add(labor.name, "Фиксированная оплата на всю партию", money(*labor.fee), *labor.fee)
			continue
		}
		if !labor.known {
			continue
		}
		var rate *model.PersonnelRate
		for i := range settings.PersonnelRates {
			if settings.PersonnelRates[i].Role == labor.role {
				rate = &settings.PersonnelRates[i]
			}
		}
		if rate != nil && rate.MonthlyAvailableHours > 0 {
			add(labor.name, "(Стоимость труда в месяц ÷ доступные часы в месяц) × "+labor.formula,
				"("+money(rate.MonthlyCost)+" ÷ "+costNumber(rate.MonthlyAvailableHours)+" ч) × "+labor.expression,
				rate.MonthlyCost/rate.MonthlyAvailableHours*labor.duration)
		} else if settings.PersonnelRates != nil {
			add(labor.name, "Ставка персонала не настроена. Расходы не включены", "0", 0)
		}
	}
	if tool := settings.Consumable; tool != nil && len(inputData) > 0 {
		if tool.ResourceMinutes > 0 {
			add("Инструмент", "Цена фрезы ÷ ресурс фрезы, мин × минуты работы на деталь × количество",
				money(tool.UnitPrice)+" ÷ "+costNumber(tool.ResourceMinutes)+" мин × "+costNumber(input.ToolUsageMinutes)+" мин × "+costNumber(q),
				tool.UnitPrice/tool.ResourceMinutes*input.ToolUsageMinutes*q)
		} else {
			add("Инструмент", "Ресурс фрезы не указан. Износ не включён", "0", 0)
		}
	}
	if company := settings.CompanyEconomics; company != nil && timeKnown {
		if company.MonthlyProductionHours > 0 {
			add("Аренда", "Аренда в месяц ÷ производственные часы в месяц × часы на деталь × количество",
				money(company.MonthlyRent)+" ÷ "+costNumber(company.MonthlyProductionHours)+" ч × "+timeExpression,
				company.MonthlyRent/company.MonthlyProductionHours*hours*q)
			if settings.Overheads != nil {
				explainSavedOverheads(out, page.CostItems, settings.Overheads, company.MonthlyProductionHours, hours, q, b.Currency)
			}
		}
		if company.MonthlyRent == 0 {
			add("Аренда", "Аренда не задана. Расходы не включены", "0", 0)
		}
	}
	part := page.Result.Part
	if part.Stock.Width == 0 {
		part = legacy.Revision.ParsedPart
	}
	explainSavedMaterial(out, b, settings, input, part)
	if timeKnown {
		for name, explanation := range out {
			usesMachineTime := false
			for _, step := range explanation.Batch {
				usesMachineTime = usesMachineTime || strings.Contains(step.Formula, "часы на деталь") || strings.Contains(step.Formula, "часы обработки одной детали")
			}
			if usesMachineTime {
				explanation.Batch = append([]costFormula{{Formula: "Минуты обработки одной детали ÷ 60 = часы обработки одной детали",
					Values: costNumber(hours*60) + " мин ÷ 60 = " + costNumber(hours) + " ч"}}, explanation.Batch...)
				out[name] = explanation
			}
		}
	}
	return out
}

func explainSavedOverheads(out map[string]costExplanation, items []model.ProductionCostItem, overheads []model.CompanyOverhead, monthlyHours, hours, q float64, currency string) {
	for _, category := range []struct{ key, name string }{{model.OverheadUtilities, "Коммунальные расходы"}, {model.OverheadOther, "Прочие накладные расходы"}} {
		var sum float64
		var terms []string
		var steps []costFormula
		for _, cost := range overheads {
			if !cost.Active || cost.Category != category.key {
				continue
			}
			periodHours := monthlyHours
			period := "месяц"
			if cost.Period == model.OverheadAnnual {
				periodHours *= 12
				period = "год"
			}
			amount := cost.Amount / periodHours * hours * q
			sum += amount
			terms = append(terms, formatMoney(amount, currency))
			steps = append(steps, costFormula{
				Formula: cost.Name + ": сумма за " + period + " ÷ производственные часы за " + period + " × часы на деталь × количество",
				Values:  fmt.Sprintf("%s ÷ %s ч × %s ч × %s = %s", formatMoney(cost.Amount, currency), costNumber(periodHours), costNumber(hours), costNumber(q), formatMoney(amount, currency)),
			})
		}
		for _, item := range items {
			if item.Name != category.name || math.Abs(item.BatchCost-sum) > 0.005 {
				continue
			}
			if len(steps) == 0 {
				steps = []costFormula{{Formula: "Активные расходы этой категории не заданы. Статья не включена", Values: "0 = " + formatMoney(item.BatchCost, currency)}}
			} else if len(steps) > 1 {
				steps = append(steps, costFormula{Formula: "Сумма расходов категории на партию", Values: strings.Join(terms, " + ") + " = " + formatMoney(item.BatchCost, currency)})
			}
			explanation := out[item.Name]
			explanation.Batch, explanation.Note = steps, ""
			out[item.Name] = explanation
		}
	}
}

func explainSavedMaterial(out map[string]costExplanation, b *model.ProductionCostBreakdown, s savedCostSettings, input application.EstimateInput, part model.Part) {
	if b.Quantity <= 0 {
		return
	}
	q := float64(b.Quantity)
	price, density := 0.0, 0.0
	if s.Material != nil {
		price, density = s.Material.PricePerKg, s.Material.Density
	}
	if s.GlobalMaterial != nil {
		price, density = s.GlobalMaterial.ReferencePriceKg, s.GlobalMaterial.Density
	}
	var steps []costFormula
	mass := input.MaterialWeightKg
	known := mass > 0
	if blank := s.StockBlank; blank != nil {
		if blank.PricePerPiece > 0 {
			if math.Abs(blank.PricePerPiece*q-b.MaterialCost) <= 0.005 {
				e := out["Материал"]
				e.Batch = []costFormula{{Formula: "Цена заготовки за штуку × количество деталей", Values: fmt.Sprintf("%s × %s = %s", formatMoney(blank.PricePerPiece, b.Currency), costNumber(q), formatMoney(b.MaterialCost, b.Currency))}}
				out["Материал"] = e
			}
			return
		}
		if density > 0 {
			var volume float64
			var expression, formula string
			switch blank.Shape {
			case model.StockBlankShapeRectangular:
				volume = blank.Length * blank.Width * blank.Height
				formula = "Длина × ширина × высота × плотность ÷ 1 000 000 000 = масса заготовки, кг"
				expression = fmt.Sprintf("%s × %s × %s мм³ × %s кг/м³ ÷ 1 000 000 000", costNumber(blank.Length), costNumber(blank.Width), costNumber(blank.Height), costNumber(density))
			case model.StockBlankShapeRound:
				volume = math.Pi * math.Pow(blank.Diameter/2, 2) * blank.Length
				formula = "π × (диаметр ÷ 2)² × длина × плотность ÷ 1 000 000 000 = масса заготовки, кг"
				expression = fmt.Sprintf("π × (%s ÷ 2)² × %s мм³ × %s кг/м³ ÷ 1 000 000 000", costNumber(blank.Diameter), costNumber(blank.Length), costNumber(density))
			}
			if volume > 0 {
				mass = volume * density / 1e9
				known = true
				steps = append(steps, costFormula{Formula: formula, Values: expression + " = " + costNumber(mass) + " кг"})
			}
		}
	} else if !known && density > 0 && part.Stock.Width > 0 && part.Stock.Length > 0 && part.Stock.Height > 0 {
		mass = part.Stock.Width * part.Stock.Length * part.Stock.Height * density / 1e9
		known = true
		steps = append(steps, costFormula{Formula: "Ширина × длина × высота × плотность ÷ 1 000 000 000 = масса заготовки, кг",
			Values: fmt.Sprintf("%s × %s × %s мм³ × %s кг/м³ ÷ 1 000 000 000 = %s кг", costNumber(part.Stock.Width), costNumber(part.Stock.Length), costNumber(part.Stock.Height), costNumber(density), costNumber(mass))})
	}
	if known && math.Abs(mass*price*q-b.MaterialCost) <= 0.005 && price > 0 {
		steps = append(steps, costFormula{Formula: "Масса заготовки на деталь × цена за килограмм × количество деталей",
			Values: fmt.Sprintf("%s кг × %s/кг × %s = %s", costNumber(mass), formatMoney(price, b.Currency), costNumber(q), formatMoney(b.MaterialCost, b.Currency))})
		e := out["Материал"]
		e.Batch = steps
		out["Материал"] = e
	}
}
