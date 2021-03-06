package main

import (
    crand "crypto/rand"
    "flag"
    "github.com/nats-io/nats.go"
    "log"
    "math"
    "math/big"
    "math/rand"
    "os"
    "runtime"
    "time"
)

type simNum struct {
    x         float64
    y         float64
    direction float64
    speed     float64
}

func main() {
    seed, _ := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
    rand.Seed(seed.Int64())
    var (
        direction   = rand.Float64() * 2 * math.Pi
        wall        float64
        speed       float64
        x           float64
        y           float64
        viewAngle   float64
        viewR       float64
        nats_server string
    )
    flag.Float64Var(&wall, "wall", 100.0, "limit x-max, y-max(min = 0)")
    flag.Float64Var(&speed, "speed", math.NaN(), "moving speed")
    flag.Float64Var(&x, "x", math.NaN(), "init x value")
    flag.Float64Var(&y, "y", math.NaN(), "init y value")
    flag.Float64Var(&viewAngle, "view_angle", math.NaN(), "init view angle in radian")
    flag.Float64Var(&viewR, "view_r", math.NaN(), "init view r")
    flag.StringVar(&nats_server, "nats_server", "nats:4222", "NATS messaging server")
    flag.Parse()
    if math.IsNaN(x) {
        x = rand.Float64() * wall
    }
    if math.IsNaN(y) {
        y = rand.Float64() * wall
    }
    if math.IsNaN(speed) {
        speed = 1.0 + (rand.Float64() * 4.0)
    }
    if math.IsNaN(viewAngle) {
        viewAngle = (math.Pi / 6.0) + (rand.Float64() * math.Pi * 5.0 / 6.0)
    }
    if math.IsNaN(viewR) {
        viewR = 3.0 * rand.Float64() * 20.0
    }

    current := simNum{x: x, y: y, direction: direction, speed: speed}
    next := current
    shortestR := viewR
    opt := []nats.Option{}
    opt = setupConnOptions(opt)
    nc, err := nats.Connect("nats://" + nats_server, opt...)
    n, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
    if err != nil {
        println("connection problem")
        panic(err)
    }
    n.Publish("agents.init", "init")
    println("init!")

    n.Subscribe("agents.report", func(msg simNum) {
        if msg == current {
            println("fromME!")
        }
        if msg.x == current.x && msg.y == current.y {
            println("check!")
        }
        if msg != current {
            diffX := msg.x - current.x
            diffY := msg.y - current.y
            r := math.Sqrt(math.Pow(diffX, 2) + math.Pow(diffY, 2))
            if (r == 0.0) {
                println("from me")
            }
            if (r != 0.0) && (r <= shortestR) {
                angle := math.Atan2(diffY, diffX)
                if math.Abs(angle-current.direction) <= viewAngle/2 {
                    diffX = diffX - current.speed * math.Cos(current.direction) + msg.speed * math.Cos(msg.direction)
                    diffY = diffY - current.speed * math.Sin(current.direction) + msg.speed * math.Sin(msg.direction)
                    futureR := math.Sqrt(math.Pow(diffX, 2) + math.Pow(diffY, 2))
                    angle = math.Atan2(diffY, diffX)
                    if angle - current.direction > 0 {
                        next.direction = current.direction - (math.Pi/2) * ((viewR - futureR) / viewR)
                        next.direction = math.Mod(next.direction, 2*math.Pi)
                        if next.direction < 0 {
                            next.direction = next.direction + 2 * math.Pi
                        }
                        shortestR = r
                    } else {
                        next.direction = current.direction + (math.Pi/2) * ((viewR - futureR) / viewR)
                        next.direction = math.Mod(next.direction, 2 * math.Pi)
                        shortestR = r
                    }
                }
            }
        }
    })
    n.Subscribe("api.move", func(msg string) {
        //println("movin!")
        next.x = current.x + next.speed*math.Sin(next.direction)
        next.y = current.y + next.speed*math.Cos(next.direction)
        for loop := 0; ; {
            loop = loop + 1
            next.x, next.direction, _ = boundCheck(next.x, wall, next.direction, true, 0)
            if next.x == math.Mod(next.x, wall) {
                break
            }
        }

        for loop := 0; ; {
            loop = loop + 1
            next.y, next.direction, _ = boundCheck(next.y, wall, next.direction, false, 0)
            if next.y == math.Mod(next.y, wall) {
                break
            }
        }
        if next.direction < 0 {
            next.direction = next.direction + 2 * math.Pi
        }
        current.x = next.x
        current.y = next.y
        current.direction = next.direction
        current.speed = next.speed
        //println(shortestR)
        shortestR = viewR
        //println("x:", current.x, "y:", current.y, "direction:", current.direction, "speed:", current.speed)
        n.Publish("agents.moved", current)
    })
    n.Subscribe("api.next", func(msg string) {
        //println("next!")
        n.Publish("agents.report", current)
    })
    n.Subscribe("api.exit", func(msg string) {
        println("EXIT!")
        os.Exit(0)
    })
    println("ok")
    if err != nil {
        panic(err)
    }
    runtime.Goexit()
}

func boundCheck(loc float64, wall float64, direction float64, isX bool, bound int) (float64, float64, int) {
    if loc > wall {
        loc = loc - (2 * (loc - wall))
        bound = bound + 1
    }
    if loc < 0 {
        loc = -loc
        bound = bound + 1
    }

    if bound%2 == 1 {
        if isX == true {
            if math.Mod(direction, math.Pi) < math.Pi/2 {
                direction = direction + math.Pi/2
            } else {
                direction = direction - math.Pi/2
            }
        } else {
            if math.Mod(direction, math.Pi) > math.Pi/2 {
                direction = direction + math.Pi/2
            } else {
                direction = direction - math.Pi/2
            }
        }
    }
    direction = math.Mod(direction, 2*math.Pi)
    return loc, direction, bound
}

func setupConnOptions(opts []nats.Option) []nats.Option {
    totalWait := 10 * time.Minute
    reconnectDelay := time.Second

    opts = append(opts, nats.ReconnectWait(reconnectDelay))
    opts = append(opts, nats.MaxReconnects(int(totalWait/reconnectDelay)))
    opts = append(opts, nats.DisconnectHandler(func(nc *nats.Conn) {
        log.Printf("Disconnected: will attempt reconnects for %.0fm", totalWait.Minutes())
    }))
    opts = append(opts, nats.ReconnectHandler(func(nc *nats.Conn) {
        log.Printf("Reconnected [%s]", nc.ConnectedUrl())
    }))
    opts = append(opts, nats.ClosedHandler(func(nc *nats.Conn) {
        log.Fatalf("Exiting: %v", nc.LastError())
    }))
    return opts
}
