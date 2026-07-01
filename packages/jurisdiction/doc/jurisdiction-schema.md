# Jurisdiction Schema

This document describes the data model used by the `jurisdiction` package to represent courts and legal authorities within the Verdex judicial reasoning platform.

---

## Jurisdiction

| Field             | Type              | Required | Description |
|-------------------|-------------------|----------|-------------|
| `id`              | `uuid.UUID`       | yes      | Globally unique identifier (UUID v4). Assigned on creation. |
| `country_code`    | `string`          | yes      | ISO 3166-1 alpha-2 country code (e.g. `"AE"`, `"PK"`, `"IN"`). Must be exactly 2 uppercase ASCII letters. |
| `country_name`    | `string`          | yes      | Full English name of the sovereign state (e.g. `"United Arab Emirates"`). |
| `court_level`     | `CourtLevel`      | yes      | Hierarchical position of the court. See [CourtLevel values](#courtlevel-values). |
| `court_name`      | `string`          | yes      | Official name of the court or judicial authority. |
| `legal_family`    | `LegalFamily`     | yes      | Primary legal tradition. See [LegalFamily values](#legalfamily-values). |
| `languages`       | `[]string`        | yes      | One or more ISO 639-1 two-letter language codes (lowercase) used in proceedings. |
| `procedural_rules`| `[]ProceduralRule`| no       | Procedural codes and statutes governing practice before this court. |
| `created_at`      | `time.Time`       | yes      | UTC timestamp of record creation. |
| `updated_at`      | `time.Time`       | yes      | UTC timestamp of last modification. |

---

## ProceduralRule

| Field         | Type     | Required | Description |
|---------------|----------|----------|-------------|
| `code`        | `string` | yes      | Short machine-readable identifier (e.g. `"CPC"`, `"FRCP"`, `"ADGM-CPR"`). |
| `name`        | `string` | yes      | Human-readable name of the procedural instrument. |
| `description` | `string` | no       | Brief overview of the rule's scope and applicability. |

---

## LegalFamily Values

| Constant                  | Wire value      | Description |
|---------------------------|-----------------|-------------|
| `LegalFamilyCommonLaw`    | `common_law`    | English common-law tradition (UK, USA, India, Nigeria, etc.). |
| `LegalFamilyCivilLaw`     | `civil_law`     | Codified civil-law systems (Napoleonic, Roman-Germanic). |
| `LegalFamilyMixed`        | `mixed`         | Blend of two or more legal traditions (e.g. common law + Islamic law). |
| `LegalFamilyIslamicLaw`   | `islamic_law`   | Primary source of law is Shari'a / Islamic jurisprudence. |

---

## CourtLevel Values

| Constant                | Wire value    | Description |
|-------------------------|---------------|-------------|
| `CourtLevelSupreme`     | `supreme`     | Apex court (Supreme Court, Federal Court, House of Lords). |
| `CourtLevelAppellate`   | `appellate`   | Intermediate appellate court sitting below the supreme court. |
| `CourtLevelHigh`        | `high`        | Superior court of first instance or record (High Court). |
| `CourtLevelDistrict`    | `district`    | District, sessions, or county-level courts. |
| `CourtLevelMagistrate`  | `magistrate`  | Magistrates', summary, or minor courts. |
| `CourtLevelSpecial`     | `special`     | Tribunals, specialist courts (family, labour, commercial, administrative, sharia personal-status), or courts outside the main hierarchy. |

---

## Validation Rules

1. `country_code` must be exactly 2 uppercase ASCII letters (ISO 3166-1 alpha-2).
2. `country_name` must not be blank.
3. `court_name` must not be blank.
4. `court_level` must be one of the recognised `CourtLevel` constants.
5. `legal_family` must be one of the recognised `LegalFamily` constants.
6. `languages` must contain at least one entry; each entry must be a 2-letter lowercase ISO 639-1 code.

---

## Seed Jurisdictions

The package ships with `SeedData()`, which returns 13 pre-populated jurisdictions:

| # | Country Code | Country         | Court                              | Legal Family | Level     |
|---|-------------|-----------------|-------------------------------------|--------------|-----------|
| 1 | AE          | UAE             | Federal Supreme Court of the UAE    | mixed        | supreme   |
| 2 | AE          | UAE             | Dubai Courts (High Civil Court)     | mixed        | high      |
| 3 | AE          | UAE             | Abu Dhabi Global Market Courts (ADGM) | common_law | special   |
| 4 | PK          | Pakistan        | Supreme Court of Pakistan           | mixed        | supreme   |
| 5 | PK          | Pakistan        | Lahore High Court                   | mixed        | high      |
| 6 | IN          | India           | Supreme Court of India              | common_law   | supreme   |
| 7 | LK          | Sri Lanka       | Supreme Court of Sri Lanka          | mixed        | supreme   |
| 8 | GB          | United Kingdom  | UK Supreme Court                    | common_law   | supreme   |
| 9 | US          | United States   | Supreme Court of the United States  | common_law   | supreme   |
|10 | EG          | Egypt           | Supreme Constitutional Court        | mixed        | supreme   |
|11 | SA          | Saudi Arabia    | Supreme Court of Saudi Arabia       | islamic_law  | supreme   |
|12 | MY          | Malaysia        | Federal Court of Malaysia           | mixed        | supreme   |
|13 | NG          | Nigeria         | Supreme Court of Nigeria            | mixed        | supreme   |

---

## Interfaces

### LookupService

Read-only access to the jurisdiction registry:

```go
type LookupService interface {
    GetByID(ctx context.Context, id uuid.UUID) (Jurisdiction, error)
    GetByCountry(ctx context.Context, countryCode string) ([]Jurisdiction, error)
    ListAll(ctx context.Context) ([]Jurisdiction, error)
    Search(ctx context.Context, query string) ([]Jurisdiction, error)
}
```

The in-memory implementation (`InMemoryLookupService`) is thread-safe and suitable for seeded data and unit tests.

### Repository

Full CRUD persistence contract:

```go
type Repository interface {
    Create(ctx context.Context, j Jurisdiction) (Jurisdiction, error)
    GetByID(ctx context.Context, id uuid.UUID) (Jurisdiction, error)
    GetByCountry(ctx context.Context, countryCode string) ([]Jurisdiction, error)
    ListAll(ctx context.Context) ([]Jurisdiction, error)
    Update(ctx context.Context, j Jurisdiction) (Jurisdiction, error)
    Delete(ctx context.Context, id uuid.UUID) error
}
```

---

## Error Sentinels

| Error                      | When returned |
|----------------------------|---------------|
| `ErrJurisdictionNotFound`  | Requested jurisdiction does not exist. |
| `ErrInvalidJurisdiction`   | Jurisdiction fails structural or business-rule validation. |
| `ErrDuplicateJurisdiction` | Create attempted for an already-existing jurisdiction. |
| `ErrCountryCodeInvalid`    | Country code is not a valid ISO 3166-1 alpha-2 code. |
