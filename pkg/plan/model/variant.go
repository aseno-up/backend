package model

// PlanVariant identifies whether a plan is contracted as a solo practitioner
// (single professional, discounted price) or as a clinic (default behavior).
type PlanVariant string

const (
	// PlanVariantClinic is the default; represents the full plan contract.
	PlanVariantClinic PlanVariant = ""
	// PlanVariantSolo represents a single-professional contract with discounted price.
	PlanVariantSolo PlanVariant = "solo"
)

// IsSolo returns true when the variant represents a solo practitioner contract.
func (v PlanVariant) IsSolo() bool {
	return v == PlanVariantSolo
}

// Normalize coerces free-form input into a known variant. Anything that is not
// explicitly "solo" is treated as clinic.
func NormalizeVariant(s string) PlanVariant {
	if PlanVariant(s) == PlanVariantSolo {
		return PlanVariantSolo
	}
	return PlanVariantClinic
}

// SoloMaxProfessionals is the fixed professional limit when the contract is solo.
// By product rule, solo always equals one professional.
const SoloMaxProfessionals = 1
