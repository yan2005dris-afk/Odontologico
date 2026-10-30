/**
 * Local TOTP generator for the seeded SIGO-Dental account (RNF-05).
 *
 * The identity service derives the 6-digit MFA code from the user's base32
 * TOTP secret. This tool reuses the very same domain function
 * (`identity/domain.GenerateTOTP`), so the code it prints is by construction
 * the code the server accepts — there is no second implementation to drift.
 *
 * Usage:
 *   go run ./cmd/totp                       # secret of the seeded dentist
 *   go run ./cmd/totp <base32-secret>       # any other secret
 *
 * The code is valid for the current 30-second step; the server also accepts
 * the previous step (drift window -30s / 0 / +30s).
 */
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/identity/domain"
)

const seededSecret = "JBSWY3DPEHPK3PXP"

func main() {
	secret := seededSecret
	if len(os.Args) > 1 {
		secret = os.Args[1]
	}

	code, err := domain.GenerateTOTP(secret, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot derive TOTP: %v\n", err)
		os.Exit(1)
	}

	secondsLeft := 30 - int(time.Now().Unix()%30)
	fmt.Printf("%s  (%ds restantes)\n", code, secondsLeft)
}
