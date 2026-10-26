package main

import (
	"encoding/hex"
	"math"
	"os"
	"strings"

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

func moneyHandle(value any) (*Money, error) {
	money, ok := value.(*Money)
	if !ok || money == nil {
		return nil, lmcode.NewHostError("money: null handle")
	}
	return money, nil
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
	h.registerConversions()
	mustRegister(lmcode.RegisterTesting(h.Registry, os.Stdout)) // Explicitly opt into print/assert_close.
	for _, cents := range []int64{125, 250, 375} {
		h.Shop.Items.Values = append(h.Shop.Items.Values, h.newMoney(cents).Obj.(*Money))
	}
	return h
}

func (h *Host) registerConversions() {
	r := h.Registry
	mustRegister(r.Function("host_int", []string{"int"}, "int", func(a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(a[0].Int), nil
	}))
	mustRegister(r.Function("host_float", []string{"float"}, "float", func(a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Float(a[0].Float), nil
	}))
	mustRegister(r.Function("host_bool", []string{"bool"}, "bool", func(a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(a[0].Bool), nil
	}))
	mustRegister(r.Function("host_bytes", []string{"string"}, "string", func(a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.String(a[0].Str), nil
	}))
	mustRegister(r.Function("host_hex", []string{"string"}, "string", func(a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.String(hex.EncodeToString([]byte(a[0].Str))), nil
	}))
	mustRegister(r.Function("host_float_case", []string{"string"}, "float", func(a []lmcode.Value) (lmcode.Value, error) {
		switch a[0].Str {
		case "-0":
			return lmcode.Float(math.Copysign(0, -1)), nil
		case "nan":
			return lmcode.Float(math.NaN()), nil
		case "inf":
			return lmcode.Float(math.Inf(1)), nil
		case "-inf":
			return lmcode.Float(math.Inf(-1)), nil
		case "large":
			return lmcode.Float(1e21), nil
		case "subnormal":
			return lmcode.Float(math.SmallestNonzeroFloat64), nil
		default:
			return lmcode.None(), lmcode.NewHostError("host_float_case: unknown case")
		}
	}))
	mustRegister(r.Function("host_echo_money", []string{"Money"}, "Money", func(a []lmcode.Value) (lmcode.Value, error) {
		return a[0], nil // Returning the same Value preserves handle identity.
	}))
	mustRegister(r.Function("host_same", []string{"Money", "Money"}, "bool", func(a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(a[0].Obj == a[1].Obj), nil
	}))
	mustRegister(r.Function("host_null", nil, "Money", func([]lmcode.Value) (lmcode.Value, error) {
		return lmcode.Object("Money", (*Money)(nil)), nil
	}))
	mustRegister(r.Function("host_is_null", []string{"Money"}, "bool", func(a []lmcode.Value) (lmcode.Value, error) {
		money, ok := a[0].Obj.(*Money)
		return lmcode.Bool(!ok || money == nil), nil
	}))
	mustRegister(r.Function("host_wrong", nil, "Money", func([]lmcode.Value) (lmcode.Value, error) {
		return lmcode.Object("Items", (*Items)(nil)), nil // The VM must reject this wrong schema type.
	}))
	mustRegister(r.Function("host_void", nil, "void", func([]lmcode.Value) (lmcode.Value, error) {
		return lmcode.None(), nil
	}))
	mustRegister(r.Function("host_str_list", []string{"StrList"}, "StrList", func(a []lmcode.Value) (lmcode.Value, error) {
		values := append(a[0].Obj.(*lmcode.StrList).Values(), "host")
		return copyStdList(r, "fn str_list() -> StrList", "method StrList.add(string) -> StrList", "StrList", values, func(v string) lmcode.Value { return lmcode.String(v) })
	}))
	mustRegister(r.Function("host_int_list", []string{"IntList"}, "IntList", func(a []lmcode.Value) (lmcode.Value, error) {
		values := append(a[0].Obj.(*lmcode.IntList).Values(), 9)
		return copyStdList(r, "fn int_list() -> IntList", "method IntList.add(int) -> IntList", "IntList", values, func(v int64) lmcode.Value { return lmcode.Int(v) })
	}))
	mustRegister(r.Function("host_float_list", []string{"FloatList"}, "FloatList", func(a []lmcode.Value) (lmcode.Value, error) {
		values := append(a[0].Obj.(*lmcode.FloatList).Values(), 0.5)
		return copyStdList(r, "fn float_list() -> FloatList", "method FloatList.add(float) -> FloatList", "FloatList", values, func(v float64) lmcode.Value { return lmcode.Float(v) })
	}))
	mustRegister(r.Function("host_str_map", []string{"StrMap"}, "StrMap", func(a []lmcode.Value) (lmcode.Value, error) {
		result, err := newStdObject(r, "fn str_map() -> StrMap")
		if err != nil {
			return lmcode.None(), err
		}
		set := r.MethodBySignature("method StrMap.set(string, string) -> StrMap")
		if set == nil {
			return lmcode.None(), lmcode.NewHostError("StrMap.set is not registered")
		}
		for _, entry := range a[0].Obj.(*lmcode.StrMap).Entries() {
			if _, err := set.Fn(result.Obj, []lmcode.Value{lmcode.String(entry.Key), lmcode.String(entry.Value)}); err != nil {
				return lmcode.None(), err
			}
		}
		if _, err := set.Fn(result.Obj, []lmcode.Value{lmcode.String("host"), lmcode.String("yes")}); err != nil {
			return lmcode.None(), err
		}
		return result, nil
	}))
	params := make([]string, 255)
	for i := range params {
		params[i] = "int"
	}
	mustRegister(r.Function("host_sum", params, "int", func(a []lmcode.Value) (lmcode.Value, error) {
		var sum uint64
		for _, value := range a {
			sum += uint64(value.Int)
		}
		return lmcode.Int(int64(sum)), nil
	}))
	longName := "host_long_" + strings.Repeat("x", 128)
	mustRegister(r.Function(longName, []string{"int"}, "int", func(a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(a[0].Int), nil
	}))
}

func newStdObject(registry *lmcode.Registry, signature string) (lmcode.Value, error) {
	constructor := registry.FunctionBySignature(signature)
	if constructor == nil {
		return lmcode.None(), lmcode.NewHostError("standard collection constructor is not registered")
	}
	return constructor.Fn(nil)
}

func copyStdList[T any](registry *lmcode.Registry, constructor, appendSignature, typeName string, values []T, value func(T) lmcode.Value) (lmcode.Value, error) {
	result, err := newStdObject(registry, constructor)
	if err != nil {
		return lmcode.None(), err
	}
	appendMethod := registry.MethodBySignature(appendSignature)
	if appendMethod == nil {
		return lmcode.None(), lmcode.NewHostError(typeName + ".add is not registered")
	}
	for _, item := range values {
		if _, err := appendMethod.Fn(result.Obj, []lmcode.Value{value(item)}); err != nil {
			return lmcode.None(), err
		}
	}
	return result, nil
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
		money, err := moneyHandle(self)
		if err != nil {
			return lmcode.None(), err
		}
		return lmcode.Int(money.Cents), nil
	}))
	mustRegister(r.Method("Money", "currency", nil, "Currency", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		if _, err := moneyHandle(self); err != nil {
			return lmcode.None(), err
		}
		return lmcode.String("USD"), nil
	}))
	mustRegister(r.Method("Money", "plus", []string{"Money"}, "Money", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		left, err := moneyHandle(self)
		if err != nil {
			return lmcode.None(), err
		}
		right, err := moneyHandle(a[0].Obj)
		if err != nil {
			return lmcode.None(), err
		}
		return h.newMoney(wrapAdd(left.Cents, right.Cents)), nil
	}))
	mustRegister(r.Method("Money", "minus", []string{"Money"}, "Money", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		left, err := moneyHandle(self)
		if err != nil {
			return lmcode.None(), err
		}
		right, err := moneyHandle(a[0].Obj)
		if err != nil {
			return lmcode.None(), err
		}
		return h.newMoney(wrapSub(left.Cents, right.Cents)), nil
	}))
	mustRegister(r.Method("Money", "times", []string{"int"}, "Money", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		money, err := moneyHandle(self)
		if err != nil {
			return lmcode.None(), err
		}
		return h.newMoney(wrapMul(money.Cents, a[0].Int)), nil
	}))
	mustRegister(r.Method("Money", "neg", nil, "Money", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		money, err := moneyHandle(self)
		if err != nil {
			return lmcode.None(), err
		}
		return h.newMoney(wrapSub(0, money.Cents)), nil
	}))
	mustRegister(r.Method("Money", "set", []string{"int"}, "Money", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		item, err := moneyHandle(self)
		if err != nil {
			return lmcode.None(), err
		}
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
		money, err := moneyHandle(a[0].Obj)
		if err != nil {
			return lmcode.None(), err
		}
		ledger := self.(*Ledger)
		ledger.Balance = wrapAdd(ledger.Balance, money.Cents)
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
