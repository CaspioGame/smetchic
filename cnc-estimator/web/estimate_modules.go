package web

import (
	"fmt"
	"slices"

	"smetchic/application"
)

func filterEstimateModuleValues(values map[string]string, active application.EstimateModules) {
	catalog := application.EstimateModuleCatalog()
	keep := map[string]bool{}
	for _, module := range catalog {
		if active.Has(module.ID) {
			for _, field := range module.Fields {
				keep[field] = true
			}
		}
	}
	for _, module := range catalog {
		for _, field := range module.Fields {
			if !keep[field] {
				delete(values, field)
			}
		}
	}
}

func applyAssistantModuleChanges(values map[string]string, reply estimateAssistantReply, updates map[string]string) ([]string, error) {
	if err := reply.AddModules.Validate(); err != nil {
		return nil, err
	}
	catalog := application.EstimateModuleCatalog()
	active := map[string]bool{}
	initial := map[string]bool{}
	for _, module := range catalog {
		active[module.ID] = values["module_"+module.ID] != ""
		initial[module.ID] = active[module.ID]
	}
	for _, id := range reply.RemoveModules {
		if _, known := active[id]; !known || reply.AddModules.Has(id) {
			return nil, fmt.Errorf("модель предложила несовместимые изменения модулей")
		}
		active[id] = false
	}
	for _, id := range reply.AddModules {
		for _, module := range catalog {
			if module.ID != id {
				continue
			}
			if module.Group != "" && !active[id] {
				for _, other := range catalog {
					if other.ID != id && other.Group == module.Group && active[other.ID] {
						active[other.ID] = false
						for _, field := range other.Fields {
							delete(values, field)
						}
					}
				}
			}
			active[id] = true
		}
	}
	for field := range updates {
		var owners []string
		for _, module := range catalog {
			if slices.Contains(module.Fields, field) {
				owners = append(owners, module.ID)
			}
		}
		if len(owners) == 0 || slices.ContainsFunc(owners, func(id string) bool { return active[id] }) {
			continue
		}
		if slices.ContainsFunc(owners, reply.RemoveModules.Has) {
			return nil, fmt.Errorf("модель предложила заполнить поле в удаляемом модуле")
		}
		active[owners[0]] = true
	}
	var selected application.EstimateModules
	var labels []string
	for _, module := range catalog {
		if active[module.ID] {
			selected = append(selected, module.ID)
			values["module_"+module.ID] = "1"
			if !initial[module.ID] {
				labels = append(labels, "Добавить модуль «"+module.Name+"»")
			}
		} else {
			delete(values, "module_"+module.ID)
			if initial[module.ID] {
				labels = append(labels, "Убрать модуль «"+module.Name+"»")
			}
		}
	}
	if err := selected.Validate(); err != nil {
		return nil, err
	}
	filterEstimateModuleValues(values, selected)
	return labels, nil
}
