package inventory

import (
	"fmt"
	"reflect"
	"sort"
)

// Change is what differs for one machine between two inventories.
type Change struct {
	ID     string
	Fields []string // names of the fields whose value differs
}

// Changes is the difference between a current inventory and a new one, by machine id.
type Changes struct {
	Added   []string
	Removed []string
	Changed []Change
}

// Empty reports whether the two inventories describe the same machines.
func (c Changes) Empty() bool { return len(c.Added) == 0 && len(c.Removed) == 0 && len(c.Changed) == 0 }

// Diff compares two inventories. Machines are matched by id (the id is permanent). The result is sorted.
func Diff(cur, next *Inventory) Changes {
	index := func(inv *Inventory) map[string]Machine {
		m := map[string]Machine{}
		if inv != nil {
			for _, x := range inv.Machines {
				m[x.ID] = x
			}
		}
		return m
	}
	a, b := index(cur), index(next)
	var c Changes
	for id := range b {
		if _, ok := a[id]; !ok {
			c.Added = append(c.Added, id)
		}
	}
	for id, x := range a {
		y, ok := b[id]
		if !ok {
			c.Removed = append(c.Removed, id)
			continue
		}
		if f := fieldsDiffer(x, y); len(f) > 0 {
			c.Changed = append(c.Changed, Change{ID: id, Fields: f})
		}
	}
	sort.Strings(c.Added)
	sort.Strings(c.Removed)
	sort.Slice(c.Changed, func(i, j int) bool { return c.Changed[i].ID < c.Changed[j].ID })
	return c
}

func fieldsDiffer(x, y Machine) []string {
	vx, vy := reflect.ValueOf(x), reflect.ValueOf(y)
	t := vx.Type()
	var out []string
	for i := 0; i < t.NumField(); i++ {
		if !reflect.DeepEqual(vx.Field(i).Interface(), vy.Field(i).Interface()) {
			out = append(out, t.Field(i).Tag.Get("toml"))
		}
	}
	return out
}

// Lines prints the changes in plain words, one per line.
func (c Changes) Lines() []string {
	var out []string
	for _, id := range c.Added {
		out = append(out, "added: "+id)
	}
	for _, id := range c.Removed {
		out = append(out, "removed: "+id)
	}
	for _, ch := range c.Changed {
		out = append(out, fmt.Sprintf("changed: %s (%s)", ch.ID, joinWords(ch.Fields)))
	}
	return out
}

func joinWords(s []string) string {
	r := ""
	for i, x := range s {
		if i > 0 {
			r += ", "
		}
		r += x
	}
	return r
}
