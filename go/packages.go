package main
import (
	"fmt"
	"math/rand"
	"math"
)
func main(){
	fmt.Println("My fav no=", rand.Intn(10)) 
	/*
	pseudo random number generator
	open interval so only generates numbers <n
	can't handle numbers <=0
	*/
	fmt.Println("Now you have %g problems. \n", math.Sqrt(7.26457513110645907))
}