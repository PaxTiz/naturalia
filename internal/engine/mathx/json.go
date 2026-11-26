package mathx

import (
	"encoding/json"
	"fmt"
)

// A Vec3 reads [x, y, z] as well as {"X": x, ...}.
func (a *Vec3) UnmarshalJSON(b []byte) error {
	var arr []float64
	if err := json.Unmarshal(b, &arr); err == nil {
		if len(arr) != 3 {
			return fmt.Errorf("a position is [x, y, z], got %d numbers", len(arr))
		}
		*a = Vec3{arr[0], arr[1], arr[2]}
		return nil
	}
	type plain Vec3
	return json.Unmarshal(b, (*plain)(a))
}
