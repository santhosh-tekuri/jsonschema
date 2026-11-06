package jsonschema

import "sort"

// oneOfDiscriminator describes a required string property whose const value can
// rule out some oneOf branches. A branch without a statically recognizable
// constraint is deliberately left unclassified and must always be evaluated.
type oneOfDiscriminator struct {
	property   string
	values     []string
	classified []bool
}

func (s *Schema) prepareOneOfDiscriminator() {
	if len(s.OneOf) < 2 {
		return
	}

	type candidate struct {
		values     []string
		classified []bool
		count      int
		distinct   map[string]struct{}
	}
	candidates := map[string]*candidate{}
	for i, branch := range s.OneOf {
		for property, value := range branch.requiredStringConsts() {
			c := candidates[property]
			if c == nil {
				c = &candidate{
					values:     make([]string, len(s.OneOf)),
					classified: make([]bool, len(s.OneOf)),
					distinct:   map[string]struct{}{},
				}
				candidates[property] = c
			}
			c.values[i] = value
			c.classified[i] = true
			c.count++
			c.distinct[value] = struct{}{}
		}
	}

	// Pick deterministically when multiple required const properties could be
	// used. Classifying more branches gives validation the best opportunity to
	// avoid work. At least two distinct values are necessary to be useful for a
	// valid instance.
	properties := make([]string, 0, len(candidates))
	for property := range candidates {
		properties = append(properties, property)
	}
	sort.Strings(properties)
	var best *candidate
	var bestProperty string
	for _, property := range properties {
		c := candidates[property]
		if len(c.distinct) < 2 {
			continue
		}
		if best == nil || c.count > best.count {
			best = c
			bestProperty = property
		}
	}
	if best != nil {
		s.oneOfDiscriminator = &oneOfDiscriminator{
			property:   bestProperty,
			values:     best.values,
			classified: best.classified,
		}
	}
}

// requiredStringConsts returns only constraints that are sufficient to prove
// that every object accepted by s has the named property and that its value is
// the returned string. It intentionally recognizes a small, conservative shape:
// required + properties + const, through local compiled $ref chains.
func (s *Schema) requiredStringConsts() map[string]string {
	constraints := map[string]string{}
	seen := map[*Schema]struct{}{}
	for s != nil {
		if _, ok := seen[s]; ok {
			break
		}
		seen[s] = struct{}{}

		for _, property := range s.Required {
			propertySchema := s.Properties[property]
			if propertySchema == nil {
				continue
			}
			if value, ok := propertySchema.localStringConst(); ok {
				constraints[property] = value
			}
		}

		if s.Ref == nil || s.resource != s.Ref.resource {
			break
		}
		s = s.Ref
	}
	return constraints
}

func (s *Schema) localStringConst() (string, bool) {
	seen := map[*Schema]struct{}{}
	for s != nil {
		if _, ok := seen[s]; ok {
			return "", false
		}
		seen[s] = struct{}{}
		if s.Const != nil {
			value, ok := (*s.Const).(string)
			return value, ok
		}
		if s.Ref == nil || s.resource != s.Ref.resource {
			return "", false
		}
		s = s.Ref
	}
	return "", false
}

func (d *oneOfDiscriminator) value(instance any) (string, bool) {
	object, ok := instance.(map[string]any)
	if !ok {
		return "", false
	}
	value, ok := object[d.property].(string)
	return value, ok
}

func (d *oneOfDiscriminator) excludes(branch int, value string) bool {
	return d.classified[branch] && d.values[branch] != value
}
