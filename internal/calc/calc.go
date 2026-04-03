package calc

import "fmt"

// TopN returns the price at 1-based position n inside side (bids or asks).
func TopN(side []float64, n int) (float64, error) {
	if n < 1 || n > len(side) {
		return 0, fmt.Errorf("top_n %d out of range (len=%d)", n, len(side))
	}
	return side[n-1], nil
}

// AvgNM returns the mean of inclusive 1-based levels [n; m].
func AvgNM(side []float64, n, m int) (float64, error) {
	if n < 1 || m < n || m > len(side) {
		return 0, fmt.Errorf("avg range [%d;%d] invalid for len=%d", n, m, len(side))
	}
	sum := 0.0
	for i := n - 1; i <= m-1; i++ {
		sum += side[i]
	}
	return sum / float64(m-n+1), nil
}
