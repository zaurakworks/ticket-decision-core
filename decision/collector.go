package decision

import "sort"

func planCollections(collectors []Collector, missingFacts []string, facts map[string]any) ([]CollectionRequest, []string) {
	providers := make(map[string]Collector)
	for _, collector := range collectors {
		for _, fact := range collector.Provides {
			providers[fact] = collector
		}
	}

	selected := make(map[string]Collector)
	unresolved := make([]string, 0)
	for _, fact := range missingFacts {
		collector, ok := providers[fact]
		if !ok {
			unresolved = append(unresolved, fact)
			continue
		}
		selected[collector.ID] = collector
	}

	ids := make([]string, 0, len(selected))
	for id := range selected {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	requests := make([]CollectionRequest, 0, len(ids))
	for _, id := range ids {
		collector := selected[id]
		parameters := make(map[string]any, len(collector.Parameters))
		ready := true
		for name, factPath := range collector.Parameters {
			value, present := lookupFact(facts, factPath)
			if !present {
				unresolved = append(unresolved, factPath)
				ready = false
				continue
			}
			parameters[name] = value
		}
		if !ready {
			continue
		}
		request := CollectionRequest{
			CollectorID: collector.ID,
			Kind:        collector.Kind,
			Instruction: collector.Instruction,
			Parameters:  parameters,
			Produces:    append([]string(nil), collector.Provides...),
		}
		sort.Strings(request.Produces)
		requests = append(requests, request)
	}
	return requests, uniqueSorted(unresolved)
}
