package application

import "smetchic/model"

const DemoCompanyID = "demo-company"

type DemoData struct {
	Company        model.Company
	Settings       model.CompanySettings
	EquipmentModel model.EquipmentModel
	Equipment      model.Equipment
}

func DefaultDemoData() DemoData {
	return DemoData{
		Company: model.Company{
			ID: DemoCompanyID, Name: "Демонстрационная компания",
		},
		Settings: model.CompanySettings{
			CompanyID: DemoCompanyID, Currency: "RUB", DefaultMarkupPercent: 50,
		},
		EquipmentModel: model.EquipmentModel{
			ID: "dekart-s4040", Type: model.EquipmentMilling,
			Manufacturer: "DeKart", Model: "S4040", MaxFeed: 5000,
		},
		Equipment: model.Equipment{
			ID: "dekart-s4040", CompanyID: DemoCompanyID,
			EquipmentModelID: "dekart-s4040", Name: "DeKart S4040",
			Status: model.EquipmentStatusActive, MachineRatePerHour: 1500,
		},
	}
}
