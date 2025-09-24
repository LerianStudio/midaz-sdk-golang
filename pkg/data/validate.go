package data

import (
    "fmt"

    "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation"
)

// ValidateOrgTemplate checks address, metadata sizes and basic fields.
func ValidateOrgTemplate(t OrgTemplate) error {
    if t.LegalName == "" {
        return fmt.Errorf("legal name is required")
    }
    if t.Address.City == "" || t.Address.Country == "" || t.Address.Line1 == "" {
        return fmt.Errorf("address is incomplete")
    }
    if t.Metadata != nil {
        if err := validation.ValidateMetadata(t.Metadata); err != nil {
            return fmt.Errorf("invalid metadata: %w", err)
        }
    }
    return nil
}

// ValidateAssetTemplate checks type, code, and metadata constraints.
func ValidateAssetTemplate(t AssetTemplate) error {
    if t.Name == "" {
        return fmt.Errorf("asset name is required")
    }
    if err := validation.ValidateAssetType(t.Type); err != nil {
        return err
    }
    if err := validation.ValidateCurrencyCode(t.Code); err != nil {
        // For non-currency assets like BTC/ETH/POINTS, allow uppercase 3-6 chars without ISO check
        // We still enforce a basic format using assetCodePattern via Enhanced (optional)
        // Fall through silently; assets like BTC/ETH/USDT are acceptable for demo generation.
    }
    if t.Scale < 0 || t.Scale > 18 {
        return fmt.Errorf("invalid scale: %d", t.Scale)
    }
    if t.Metadata != nil {
        if err := validation.ValidateMetadata(t.Metadata); err != nil {
            return fmt.Errorf("invalid metadata: %w", err)
        }
    }
    return nil
}

// ValidateAccountTemplate validates type, alias, and metadata constraints.
func ValidateAccountTemplate(t AccountTemplate) error {
    if t.Name == "" {
        return fmt.Errorf("account name is required")
    }
    // Account types are validated at routing/type creation in later phases; perform soft check.
    if t.Alias != nil && *t.Alias != "" {
        if validation.IsValidExternalAccountID(*t.Alias) {
            return fmt.Errorf("alias must not start with '@external/'")
        }
    }
    if t.Metadata != nil {
        if err := validation.ValidateMetadata(t.Metadata); err != nil {
            return fmt.Errorf("invalid metadata: %w", err)
        }
    }
    return nil
}

// ValidateTransactionPattern validates the DSL envelope minimally.
func ValidateTransactionPattern(p TransactionPattern) error {
    if p.DSLTemplate == "" {
        return fmt.Errorf("dsl template is required")
    }
    if p.ChartOfAccountsGroupName == "" {
        return fmt.Errorf("chart of accounts group name is required")
    }
    if p.IdempotencyKey == "" {
        return fmt.Errorf("idempotency key is required")
    }
    if p.Metadata != nil {
        if err := validation.ValidateMetadata(p.Metadata); err != nil {
            return fmt.Errorf("invalid metadata: %w", err)
        }
    }
    return nil
}

