package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Generator генерирует последовательность чисел 1,2,3 и т.д. и отправляет их в канал ch, вызывает функцию для подсчета метрик.
func generator(ctx context.Context, ch chan<- int64, fn func(int64)) {
	defer close(ch)
	var count int64 = 1

	for {
		select {
		case ch <- count:
			fn(count)
			count++
		case <-ctx.Done():
			return
		}
	}
}

// Worker читает число из канала in и пишет его в канал out.
func worker(in <-chan int64, out chan<- int64) {
	defer close(out)

	for elem := range in {
		out <- elem
		time.Sleep(time.Millisecond)
	}
}

// FanIn передает значения из множества каналов в один, вызывает функцию для подсчета метрик.
func fanIn(fn func(inputNo int), inputs ...chan int64) chan int64 {
	var wg sync.WaitGroup
	var out = make(chan int64, len(inputs))

	for inputNo, input := range inputs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for value := range input {
				out <- value
				fn(inputNo)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// Metrics содержит в себе метрики, используемые для проверки корректности работы горутин.
type Metrics struct {
	inputSum   int64   // Сумма сгенерированных чисел.
	inputCount int64   // Количество сгенерированных чисел.
	amounts    []int64 // Слайс, в который собирается статистика по горутинам.
}

// CompareAndPrintMetrics сравнивает метрики с результатом и выводит значения в терминал.
func compareAndPrintMetrics(metrics *Metrics, sum, count int64) {
	fmt.Println("Количество чисел", metrics.inputCount, count) //nolint:forbidigo // it's metrics
	fmt.Println("Сумма чисел", metrics.inputSum, sum)          //nolint:forbidigo // it's metrics
	fmt.Println("Разбивка по каналам", metrics.amounts)        //nolint:forbidigo // it's metrics

	if metrics.inputSum != sum {
		log.Panicf("Ошибка: суммы чисел не равны: %d != %d\n", metrics.inputSum, sum)
	}
	if metrics.inputCount != count {
		log.Panicf("Ошибка: количество чисел не равно: %d != %d\n", metrics.inputCount, count)
	}
	for _, v := range metrics.amounts {
		metrics.inputCount -= v
	}
	if metrics.inputCount != 0 {
		log.Panicf("Ошибка: разделение чисел по каналам неверное\n")
	}
}

func main() {
	const numOut = 5 // Количество обрабатывающих горутин и каналов.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	metrics := &Metrics{
		inputSum:   0,
		inputCount: 0,
		amounts:    make([]int64, numOut),
	}

	var chIn = make(chan int64)
	go generator(ctx, chIn, func(i int64) {
		atomic.AddInt64(&metrics.inputSum, i)
		atomic.AddInt64(&metrics.inputCount, 1)
	})

	var outs = make([]chan int64, numOut)
	for i := range numOut {
		outs[i] = make(chan int64)
		go worker(chIn, outs[i])
	}

	var chOut = fanIn(func(inputNo int) {
		atomic.AddInt64(&metrics.amounts[inputNo], 1)
	}, outs...)

	var countOut int64
	var sumOut int64
	for val := range chOut {
		countOut++
		sumOut += val
	}

	compareAndPrintMetrics(metrics, sumOut, countOut)
}
