package main

import "testing"

func TestMerge_LongGeneratedContent(t *testing.T) {
	out := mergeContent("foo\n", "baaaaaar\n", 4)
	if out != "" {
		t.Fatal(out)
	}
}

func TestMerge_LongOrigin(t *testing.T) {
	out := mergeContent("fooo\n", "bar\n", 6)
	if out != "bar\n" {
		t.Fatal(out)
	}
}

func TestMerge_OnlyGenerated(t *testing.T) {
	out := mergeContent("fo\n", "bar\n", 4)
	if out != "bar\n" {
		t.Fatal(out)
	}
}

func TestMerge_AllContained(t *testing.T) {
	out := mergeContent("foo\n", "bar\n", 8)
	if out != "foo\nbar\n" {
		t.Fatal(out)
	}
}

func TestMerge_TakeALineFromOrigin(t *testing.T) {
	out := mergeContent("foo\nbar\n", "foo\n", 8)
	if out != "bar\nfoo\n" {
		t.Fatal(out)
	}
}

func TestMerge_TakeALineFromLessOrigin(t *testing.T) {
	out := mergeContent("foo\nbar\n", "foo\n", 10)
	if out != "bar\nfoo\n" {
		t.Fatal(out)
	}
}

func TestMerge_TakeALineFromLessGenerated(t *testing.T) {
	out := mergeContent("foobar\n", "foobar\nbar\nfoo\n", 10)
	if out != "bar\nfoo\n" {
		t.Fatal(out)
	}
}
