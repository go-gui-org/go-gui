package gui

// maxDataConvLen caps convenience-field slice conversions
// (RowsData, OptionsData, etc.) to prevent unbounded allocation.
const maxDataConvLen = 100_000
