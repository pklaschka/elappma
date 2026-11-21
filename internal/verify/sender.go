package verify

// SenderCheck compares the sender in the package with the one known so far.
//
//	now.BundleID == "" and known.IsZero() -> skip "unreadable"
//	known.IsZero()                        -> skip "first" (Got now)
//	known.Equal(now)                      -> pass "same" (Got now)
//	accepted                              -> pass "accepted" (Want known, Got now)
//	else                                  -> fail "changed" (Want known, Got now)
//
// Want and Got hold Sender.String(); Localize writes them in the report's
// language from Report.Before and After.
func SenderCheck(known, now Sender, accepted bool) Check {
	c := Check{ID: CheckSender}
	switch {
	case now.BundleID == "" && known.IsZero():
		c.Result, c.Code = Skip, "unreadable"
	case known.IsZero():
		c.Result, c.Code, c.Got = Skip, "first", now.String()
	case known.Equal(now):
		c.Result, c.Code, c.Got = Pass, "same", now.String()
	case accepted:
		c.Result, c.Code, c.Want, c.Got = Pass, "accepted", known.String(), now.String()
	default:
		c.Result, c.Code, c.Want, c.Got = Fail, "changed", known.String(), now.String()
	}
	return c
}
