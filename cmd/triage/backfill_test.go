package main

import "testing"

func TestOrphanAfter(t *testing.T) {
	body := "You can complain. One caution: an inspector can condemn the home (order it closed). Then everyone must leave."
	if !orphanAfter(body, []string{"One caution: an inspector can condemn the home (order it closed)."}) {
		t.Fatal("a short pointer sentence after the removed line is an orphan")
	}
	body = "You can sue. You get this money only if you win your case and your landlord pays. A court can order your landlord to pay your losses and lawyer fees."
	if orphanAfter(body, []string{"You get this money only if you win your case and your landlord pays."}) {
		t.Fatal("a full sentence after the removed line stands on its own")
	}
	if orphanAfter("Last line goes. You get this money only if you win.", []string{"You get this money only if you win."}) {
		t.Fatal("nothing follows the removed line")
	}
}
