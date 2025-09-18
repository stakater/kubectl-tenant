// internal/client/tenant_printer.go
package client

import "fmt"

// PrintTenantSpec prints spec in human-readable indented format
func PrintTenantSpec(spec map[string]interface{}, indent string) {
	for k, v := range spec {
		switch val := v.(type) {
		case map[string]interface{}:
			fmt.Printf("%s%s:\n", indent, k)
			PrintTenantSpec(val, indent+"  ")
		case []interface{}:
			fmt.Printf("%s%s:\n", indent, k)
			for _, item := range val {
				switch it := item.(type) {
				case map[string]interface{}:
					fmt.Printf("%s  -\n", indent)
					PrintTenantSpec(it, indent+"    ")
				case string:
					fmt.Printf("%s  - %s\n", indent, it)
				default:
					fmt.Printf("%s  - %v\n", indent, it)
				}
			}
		case string, bool, int, float64:
			fmt.Printf("%s%s: %v\n", indent, k, val)
		default:
			fmt.Printf("%s%s: %v (type: %T)\n", indent, k, val, val)
		}
	}
}
