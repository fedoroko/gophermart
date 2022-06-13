package validation

// IsValid luhn algorithm
func IsValid(number int64) bool {
	return (number%10+checksum(number/10))%10 == 0
}

func checksum(number int64) int64 {
	var count int64

	for i := 0; number > 0; i++ {
		cur := number % 10

		if i%2 == 0 { // even
			cur = cur * 2
			if cur > 9 {
				cur = cur%10 + cur/10
			}
		}

		count += cur
		number = number / 10
	}
	return count % 10
}
