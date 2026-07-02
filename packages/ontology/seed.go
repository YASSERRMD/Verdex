package ontology

import (
	"fmt"

	"github.com/YASSERRMD/verdex/packages/category"
)

// coreConceptSeed describes one core legal concept to seed for a
// top-level category, before the concept's final ID is derived.
type coreConceptSeed struct {
	slug        string
	name        string
	description string
}

// coreConceptsByCategory holds 3-5 representative core legal concepts for
// each top-level category.CategoryCode recognized by
// category.DefaultTopLevelCategories. This is deliberately data, not a
// hard-coded exhaustive legal taxonomy: downstream jurisdiction overlays
// (overlay.go) add or modify concepts on top of this core set.
var coreConceptsByCategory = map[category.CategoryCode][]coreConceptSeed{
	category.CodeCivil: {
		{"negligence", "Negligence", "Failure to exercise the standard of care a reasonable person would exercise in similar circumstances, causing harm."},
		{"breach-of-contract", "Breach of Contract", "Failure to perform an obligation required under the terms of a valid contract."},
		{"liability", "Liability", "Legal responsibility for one's acts or omissions, giving rise to a duty to remedy harm caused."},
		{"damages", "Damages", "Monetary compensation sought or awarded to a party for loss or injury caused by another."},
		{"tort", "Tort", "A civil wrong, other than breach of contract, that causes injury and gives rise to legal liability."},
	},
	category.CodeCriminal: {
		{"intent", "Intent", "The mental state (mens rea) of purposefully engaging in conduct or causing a result."},
		{"actus-reus", "Actus Reus", "The physical, voluntary act or omission that constitutes the conduct element of a crime."},
		{"self-defense", "Self-Defense", "A justification defense permitting reasonable force to protect oneself from imminent harm."},
		{"burden-of-proof", "Burden of Proof", "The obligation to produce evidence sufficient to prove a fact or claim to the required standard."},
		{"aggravating-circumstance", "Aggravating Circumstance", "A fact or circumstance that increases the severity or culpability of an offense."},
	},
	category.CodeDomesticViolence: {
		{"protective-order", "Protective Order", "A court order restraining a person from contacting or approaching another for their safety."},
		{"coercive-control", "Coercive Control", "A pattern of controlling, intimidating, or isolating behavior directed at a partner or family member."},
		{"pattern-of-abuse", "Pattern of Abuse", "A recurring course of conduct establishing an ongoing dynamic of harm rather than an isolated incident."},
		{"custody-risk", "Custody Risk", "A factor bearing on whether a parent's conduct poses a risk that should affect custody arrangements."},
	},
	category.CodeConsumer: {
		{"unfair-trade-practice", "Unfair Trade Practice", "A deceptive, misleading, or unconscionable act or practice in trade or commerce."},
		{"warranty", "Warranty", "An assurance by a seller regarding the quality, condition, or performance of goods or services."},
		{"consumer-protection", "Consumer Protection", "Legal safeguards ensuring fair treatment, accurate information, and remedies for consumers."},
		{"misrepresentation", "Misrepresentation", "A false statement of fact made to induce another party to enter a transaction."},
	},
	category.CodeFamily: {
		{"custody", "Custody", "The legal right and responsibility to care for and make decisions regarding a child."},
		{"consent", "Consent", "Voluntary and informed agreement to an act, arrangement, or legal relationship."},
		{"division-of-assets", "Division of Assets", "The allocation of marital or shared property between parties, typically upon separation or divorce."},
		{"alimony", "Alimony", "Court-ordered financial support paid by one former spouse to another."},
		{"guardianship", "Guardianship", "Legal authority and responsibility granted to a person to care for another who cannot care for themselves."},
	},
	category.CodeCommercial: {
		{"breach-of-contract", "Breach of Contract", "Failure to perform an obligation required under the terms of a valid commercial contract."},
		{"good-faith", "Good Faith", "An implied duty to deal honestly and fairly in the performance and enforcement of a contract."},
		{"fiduciary-duty", "Fiduciary Duty", "An obligation to act in the best interest of another party due to a relationship of trust."},
		{"indemnification", "Indemnification", "A contractual obligation to compensate another party for specified losses or liabilities."},
	},
	category.CodeLabor: {
		{"wrongful-termination", "Wrongful Termination", "The unlawful dismissal of an employee in violation of contract, statute, or public policy."},
		{"workplace-discrimination", "Workplace Discrimination", "Unfavorable treatment of an employee based on a legally protected characteristic."},
		{"unpaid-wages", "Unpaid Wages", "Compensation lawfully owed to an employee for work performed but not paid."},
		{"collective-bargaining", "Collective Bargaining", "Negotiation between an employer and organized employees over terms of employment."},
	},
}

// SeedCoreConcepts builds a representative core Concept set for the
// top-level categories present in taxonomy: for each of civil, criminal,
// domestic-violence, consumer, family, commercial, and labor, it emits
// 3-5 core legal Concepts with descriptions and CategoryCodes populated.
// Concepts for categories not present in taxonomy (for any jurisdiction)
// are omitted. Returned in a stable order (grouped by
// category.DefaultTopLevelCategories order, then seed declaration order).
func SeedCoreConcepts(taxonomy category.Taxonomy) []Concept {
	present := map[category.CategoryCode]bool{}
	for _, cats := range taxonomy {
		for code := range cats {
			present[code] = true
		}
	}

	var out []Concept
	for _, cat := range category.DefaultTopLevelCategories() {
		if !present[cat.Code] {
			continue
		}
		seeds, ok := coreConceptsByCategory[cat.Code]
		if !ok {
			continue
		}
		for _, s := range seeds {
			out = append(out, Concept{
				ID:            fmt.Sprintf("%s:%s", cat.Code, s.slug),
				Name:          s.name,
				Description:   s.description,
				CategoryCodes: []string{string(cat.Code)},
			})
		}
	}
	return out
}
