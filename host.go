package main

import (
	"math"
	"os"

	"github.com/lmctl-ai/lmcode-go"
)

// Money is a domain object; DSL handles point to the *Money, never to a copy.
type Money struct{ Cents int64 }

// Items implements the host-owned collection protocol len()/at(int).
type Items struct{ Values []*Money }

type Shop struct{ Items *Items }

type Ledger struct {
	Balance int64
	Entries int64
	Note    string
}

// Host owns every domain handle returned during its process lifetime.
type Host struct {
	Registry *lmcode.Registry
	Shop     *Shop
	Ledger   *Ledger
	money    []*Money
}

func wrapAdd(a, b int64) int64 { return int64(uint64(a) + uint64(b)) }
func wrapSub(a, b int64) int64 { return int64(uint64(a) - uint64(b)) }
func wrapMul(a, b int64) int64 { return int64(uint64(a) * uint64(b)) }

func mustRegister(err error) {
	if err != nil {
		panic(err) // Duplicate host declarations are a programming error, not a DSL failure.
	}
}

func (h *Host) newMoney(cents int64) lmcode.Value {
	item := &Money{Cents: cents}
	h.money = append(h.money, item)
	return lmcode.Object("Money", item)
}

func (h *Host) units(amount float64) (lmcode.Value, error) {
	// math.Round matches the C++ reference: ties go away from zero.
	cents := math.Round(amount * 100)
	const limit = 0x1p63
	if math.IsNaN(cents) || math.IsInf(cents, 0) || cents < -limit || cents >= limit {
		return lmcode.None(), lmcode.NewHostError("money: amount out of range")
	}
	return h.newMoney(int64(cents)), nil
}

func NewHost() *Host {
	h := &Host{
		Registry: lmcode.NewRegistry(), // Default registry installs string/math/collection/file std.
		Shop:     &Shop{Items: &Items{}},
		Ledger:   &Ledger{},
	}
	mustRegister(h.Registry.Context("shop", "Shop"))
	mustRegister(h.Registry.Context("ledger", "Ledger"))
	h.registerShop()
	mustRegister(lmcode.RegisterTesting(h.Registry, os.Stdout)) // Explicitly opt into print/assert_close.
	for _, cents := range []int64{125, 250, 375} {
		h.Shop.Items.Values = append(h.Shop.Items.Values, h.newMoney(cents).Obj.(*Money))
	}
	return h
}

func (h *Host) registerShop() {
	r := h.Registry

	// Function overloads use distinct parameter tuples; int is cents, float is dollars.
	mustRegister(r.Function("money", []string{"int"}, "Money", func(a []lmcode.Value) (lmcode.Value, error) {
		return h.newMoney(a[0].Int), nil
	}))
	mustRegister(r.Function("money", []string{"float"}, "Money", func(a []lmcode.Value) (lmcode.Value, error) {
		return h.units(a[0].Float)
	}))
	mustRegister(r.Function("fee", []string{"int"}, "int", func(a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(wrapAdd(a[0].Int, 5)), nil
	}))
	mustRegister(r.Function("sample_tmp", nil, "string", func([]lmcode.Value) (lmcode.Value, error) {
		path, ok := os.LookupEnv("SAMPLE_TMP")
		if !ok || path == "" {
			return lmcode.None(), lmcode.NewHostError("SAMPLE_TMP is required")
		}
		return lmcode.String(path), nil
	}))

	// A schema enum crosses the native boundary as its string value.
	mustRegister(r.Method("Money", "amount", nil, "int", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(self.(*Money).Cents), nil
	}))
	mustRegister(r.Method("Money", "currency", nil, "Currency", func(any, []lmcode.Value) (lmcode.Value, error) {
		return lmcode.String("USD"), nil
	}))
	mustRegister(r.Method("Money", "plus", []string{"Money"}, "Money", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return h.newMoney(wrapAdd(self.(*Money).Cents, a[0].Obj.(*Money).Cents)), nil
	}))
	mustRegister(r.Method("Money", "minus", []string{"Money"}, "Money", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return h.newMoney(wrapSub(self.(*Money).Cents, a[0].Obj.(*Money).Cents)), nil
	}))
	mustRegister(r.Method("Money", "times", []string{"int"}, "Money", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return h.newMoney(wrapMul(self.(*Money).Cents, a[0].Int)), nil
	}))
	mustRegister(r.Method("Money", "neg", nil, "Money", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return h.newMoney(wrapSub(0, self.(*Money).Cents)), nil
	}))
	mustRegister(r.Method("Money", "set", []string{"int"}, "Money", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		item := self.(*Money)
		item.Cents = a[0].Int
		return lmcode.Object("Money", item), nil // returns:self is the same pointer.
	}))

	// The VM's foreach protocol snapshots len(), then calls at(i) for each item.
	mustRegister(r.Method("Items", "len", nil, "int", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(int64(len(self.(*Items).Values))), nil
	}))
	mustRegister(r.Method("Items", "at", []string{"int"}, "Money", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		items := self.(*Items).Values
		i := a[0].Int
		if i < 0 || i >= int64(len(items)) {
			return lmcode.None(), lmcode.NewHostError("items: index out of range")
		}
		return lmcode.Object("Money", items[int(i)]), nil
	}))
	mustRegister(r.Method("Shop", "items", nil, "Items", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Object("Items", self.(*Shop).Items), nil
	}))
	mustRegister(r.Method("Shop", "total", nil, "Money", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		var total int64
		for _, item := range self.(*Shop).Items.Values {
			total = wrapAdd(total, item.Cents)
		}
		return h.newMoney(total), nil
	}))
	mustRegister(r.Method("Shop", "price", []string{"int"}, "Money", func(_ any, a []lmcode.Value) (lmcode.Value, error) {
		return h.newMoney(a[0].Int), nil
	}))
	mustRegister(r.Method("Shop", "price", []string{"float"}, "Money", func(_ any, a []lmcode.Value) (lmcode.Value, error) {
		return h.units(a[0].Float)
	}))
	mustRegister(r.Method("Shop", "currency", nil, "Currency", func(any, []lmcode.Value) (lmcode.Value, error) {
		return lmcode.String("USD"), nil
	}))
	mustRegister(r.Method("Shop", "label", nil, "string", func(any, []lmcode.Value) (lmcode.Value, error) {
		return lmcode.String("sample shop"), nil
	}))

	mustRegister(r.Method("Ledger", "record", []string{"Money"}, "Ledger", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		ledger := self.(*Ledger)
		ledger.Balance = wrapAdd(ledger.Balance, a[0].Obj.(*Money).Cents)
		ledger.Entries = wrapAdd(ledger.Entries, 1)
		return lmcode.Object("Ledger", ledger), nil
	}))
	mustRegister(r.Method("Ledger", "balance", nil, "Money", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return h.newMoney(self.(*Ledger).Balance), nil
	}))
	mustRegister(r.Method("Ledger", "count", nil, "int", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(self.(*Ledger).Entries), nil
	}))
	mustRegister(r.Method("Ledger", "note", []string{"string"}, "void", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		self.(*Ledger).Note = a[0].Str
		return lmcode.None(), nil
	}))
	mustRegister(r.Method("Ledger", "note", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.String(self.(*Ledger).Note), nil
	}))
	mustRegister(r.Method("Ledger", "reject", []string{"string"}, "void", func(_ any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.None(), lmcode.NewHostError(a[0].Str)
	}))
}
