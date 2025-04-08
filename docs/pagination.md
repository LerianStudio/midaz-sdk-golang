# Pagination in the Midaz Go SDK

The Midaz Go SDK provides a standardized pagination system that makes it easy to work with large collections of resources. This document explains how to use the pagination features effectively.

## Table of Contents

- [Basic Concepts](#basic-concepts)
- [Pagination Options](#pagination-options)
- [Working with Paginated Results](#working-with-paginated-results)
- [Pagination Helpers](#pagination-helpers)
- [Examples](#examples)

## Basic Concepts

The SDK supports two primary pagination methods:

1. **Offset-based Pagination**: Uses `limit` and `offset` parameters to specify how many items to return and where to start.
2. **Cursor-based Pagination**: Uses a cursor to navigate through large datasets efficiently.

By default, the SDK uses offset-based pagination, but it maintains compatibility with cursor-based pagination where appropriate.

## Pagination Options

To control pagination, use the `ListOptions` structure and its helper methods:

```go
// Create options with default values
options := models.NewListOptions()

// Customize as needed
options.WithLimit(10).               // Set items per page
       WithOffset(20).               // Start from item 20
       WithOrderBy("createdAt").     // Sort by creation date
       WithOrderDirection(models.SortDescending) // Sort newest first
```

### Available Settings

| Method | Description |
|--------|-------------|
| `WithLimit(int)` | Sets the maximum number of items per page |
| `WithOffset(int)` | Sets the starting position for pagination |
| `WithPage(int)` | Sets the page number (backward compatibility) |
| `WithCursor(string)` | Sets the cursor for cursor-based pagination |
| `WithOrderBy(string)` | Sets the field to sort by |
| `WithOrderDirection(SortDirection)` | Sets the sort direction (SortAscending or SortDescending) |
| `WithFilter(string, string)` | Adds a filter criteria |
| `WithFilters(map[string]string)` | Sets multiple filters at once |
| `WithDateRange(string, string)` | Sets date range filters |
| `WithAdditionalParam(string, string)` | Adds a custom query parameter |

### Constants

The SDK defines helpful constants for pagination:

```go
// Default values
models.DefaultLimit       // Default items per page (10)
models.MaxLimit          // Maximum items per page (100)
models.DefaultOffset     // Default starting position (0)
models.DefaultPage       // Default page number (1)

// Sort directions
models.SortAscending     // Ascending order ("asc")
models.SortDescending    // Descending order ("desc")
```

## Working with Paginated Results

List operations return a `ListResponse` containing:

1. The collection of items for the current page
2. Pagination metadata in the `Pagination` field

```go
// Fetch the first page
response, err := client.Accounts.ListAccounts(ctx, orgID, ledgerID, options)
if err != nil {
    // Handle error
}

// Access the items for this page
for _, account := range response.Items {
    // Process each account
}

// Access pagination information
pagination := response.Pagination
fmt.Printf("Showing %d of %d total accounts (page %d of %d)",
    len(response.Items),
    pagination.Total,
    pagination.CurrentPage(),
    pagination.TotalPages())
```

## Pagination Helpers

The `Pagination` structure provides several helper methods:

### Navigation Helpers

| Method | Description |
|--------|-------------|
| `HasMorePages()` | Returns true if there are more pages available |
| `HasPrevPage()` | Returns true if there is a previous page |
| `HasNextPage()` | Returns true if there is a next page |
| `NextPageOptions()` | Returns options for fetching the next page |
| `PrevPageOptions()` | Returns options for fetching the previous page |

### Information Helpers

| Method | Description |
|--------|-------------|
| `CurrentPage()` | Returns the current page number (1-based) |
| `TotalPages()` | Returns the total number of pages |

## Examples

### Basic Pagination

```go
// Create pagination options
options := models.NewListOptions().WithLimit(10)

// Fetch the first page
accounts, err := client.Accounts.ListAccounts(ctx, orgID, ledgerID, options)
if err != nil {
    return err
}

// Process the first page
for _, account := range accounts.Items {
    // Process each account
}

// Check if there are more pages
if accounts.Pagination.HasNextPage() {
    // Get options for the next page
    nextPageOptions := accounts.Pagination.NextPageOptions()
    
    // Fetch the next page
    nextPage, err := client.Accounts.ListAccounts(ctx, orgID, ledgerID, nextPageOptions)
    if err != nil {
        return err
    }
    
    // Process the next page
    for _, account := range nextPage.Items {
        // Process each account
    }
}
```

### Iterating Through All Pages

```go
// Initial pagination options
options := models.NewListOptions().WithLimit(25)

// Iterate through all pages
for {
    // Fetch the current page
    page, err := client.Accounts.ListAccounts(ctx, orgID, ledgerID, options)
    if err != nil {
        return err
    }
    
    // Process items on this page
    for _, account := range page.Items {
        // Process each account
    }
    
    // Check if we've reached the last page
    if !page.Pagination.HasNextPage() {
        break
    }
    
    // Update options for the next page
    options = page.Pagination.NextPageOptions()
}
```

### Filtering and Sorting

```go
// Create options with filtering and sorting
options := models.NewListOptions().
    WithLimit(20).
    WithOrderBy("name").
    WithOrderDirection(models.SortAscending).
    WithFilter("status", models.StatusActive).
    WithFilter("type", "ASSET").
    WithDateRange("2023-01-01", "2023-12-31")

// Fetch accounts matching the criteria
accounts, err := client.Accounts.ListAccounts(ctx, orgID, ledgerID, options)
if err != nil {
    return err
}

// Process the accounts
for _, account := range accounts.Items {
    // Process each account
}
```

## Best Practices

1. Use `NewListOptions()` to create options with default values
2. Always specify a reasonable limit to avoid large result sets
3. Validate user-provided limit values to prevent excessive requests
4. Use the helper methods (HasNextPage, NextPageOptions) for pagination
5. Prefer offset-based pagination for simple use cases
6. Use cursor-based pagination (when available) for efficient navigation through large datasets