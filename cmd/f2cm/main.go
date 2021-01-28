package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

var waitSig = flag.Bool("w", false, "Sleep until got exit signals")

const usage = `expected argument should be "/absolute path=>namespace/cm" or "namespace/cm=>/absolute path".`

func main() {
	flag.Parse()
	var syncPairs []syncPair
	parseCM := func(arg string) (ns, name string) {
		cm := strings.Split(arg, "/")
		if len(cm) != 2 {
			fmt.Fprintln(os.Stderr, usage)
			panic(arg)
		}

		return cm[0], cm[1]
	}

	for _, arg := range flag.Args() {
		pairs := strings.Split(arg, "=>")
		if len(pairs) != 2 {
			fmt.Fprintln(os.Stderr, usage)
			panic(arg)
		}

		pairs[0] = strings.TrimSpace(pairs[0])
		pairs[1] = strings.TrimSpace(pairs[1])
		if filepath.IsAbs(pairs[0]) {
			ns, cm := parseCM(pairs[1])
			syncPairs = append(syncPairs, syncPair{
				Src:       pairs[0],
				Dst:       cm,
				Namespace: ns,
				Direction: F2CM,
			})

			continue
		}

		if !filepath.IsAbs(pairs[1]) {
			fmt.Fprintln(os.Stderr, usage)
			panic(arg)
		}

		ns, cm := parseCM(pairs[0])
		syncPairs = append(syncPairs, syncPair{
			Src:       cm,
			Dst:       pairs[1],
			Namespace: ns,
			Direction: CM2F,
		})
	}

	if *waitSig {
		signCh := make(chan os.Signal, 1)
		signal.Notify(signCh, os.Interrupt, syscall.SIGTERM)
		<-signCh
		signal.Stop(signCh)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return
	}

	for _, p := range syncPairs {
		p.Sync(clientset)
	}
}

type syncDirection int

const (
	F2CM syncDirection = 1
	CM2F syncDirection = 2
)

type syncPair struct {
	Src       string
	Dst       string
	Namespace string
	Direction syncDirection
}

func mergeContent(origin, gen string, sizeUpperBound int) (out string) {
	if len(gen) == 0 {
		return origin
	}

	b := strings.Builder{}
	b.Grow(len(origin) + len(gen))

	if len(gen) > sizeUpperBound {
		b.WriteString(gen[len(gen)-sizeUpperBound-1:])
	} else if len(origin)+len(gen) > sizeUpperBound {
		sep := len(origin) + len(gen) - sizeUpperBound
		b.WriteString(origin[sep-1:])
		b.WriteString(gen)
	} else {
		b.WriteByte('\n')
		b.WriteString(origin)
		b.WriteString(gen)
	}

	out = b.String()
	if out[0] != 0x0a {
		nextLine := strings.IndexByte(out[1:], 0x0a)
		if nextLine >= 0 {
			out = out[1+nextLine+1:]
		} else {
			out = out[1:]
		}
	} else {
		out = out[1:]
	}

	return
}

func (p syncPair) Sync(clientSet *kubernetes.Clientset) {
	switch p.Direction {
	case F2CM:
		contentMap := make(map[string]string)
		readFile := func(name string) string {
			if v, found := contentMap[name]; found {
				return v
			}

			bytes, err := ioutil.ReadFile(filepath.Join(p.Src, name))
			if err != nil {
				fmt.Fprintf(os.Stderr, "can't read file %s : %s", filepath.Join(p.Src, name), err)
				panic(err)
			}

			contentMap[name] = string(bytes)
			return string(bytes)
		}

		const fileSizeUpperBound = 1 << 20
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			cm, err := clientSet.CoreV1().ConfigMaps(p.Namespace).Get(context.TODO(), p.Dst, metav1.GetOptions{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "can't get cm %s/%s: %s", p.Namespace, p.Dst, err)
				return err
			}

			for name, content := range cm.Data {
				cm.Data[name] = mergeContent(content, readFile(name), fileSizeUpperBound)
			}

			_, err = clientSet.CoreV1().ConfigMaps(p.Namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
			return err
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "can't update cm %s/%s: %s", p.Namespace, p.Dst, err)
			panic(err)
		}

	case CM2F:
		cm, err := clientSet.CoreV1().ConfigMaps(p.Namespace).Get(context.TODO(), p.Src, metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "can't get cm %s/%s: %s", p.Namespace, p.Src, err)
			panic(err)
		}

		for name, content := range cm.Data {
			err := ioutil.WriteFile(filepath.Join(p.Dst, name), []byte(content), 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "can't write file %s : %s", filepath.Join(p.Dst, name), err)
				panic(err)
			}
		}
	default:
		panic(p.Direction)
	}
}
