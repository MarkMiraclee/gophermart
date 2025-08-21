package luhn

import "strconv"

func IsValid(number string) bool {
	var sum int
	nDigits := len(number)
	parity := nDigits % 2

	for i, d := range number {
		digit, err := strconv.Atoi(string(d))
		if err != nil {
			return false
		}

		if i%2 == parity {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}
	return sum%10 == 0
}
