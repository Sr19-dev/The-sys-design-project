package main // coded declaration that points the compiler to start scaning the program from this line
import ( "fmt" //short for format handles I/O formatting
		"time"
)
func main(){
	fmt.Println("hello, world!")//p should be capital
	fmt.Println("Thus began a tour of go.")
	fmt.Println("The time is now:", time.Now()) //Time.Now() uses both wall time(real world time), and a monotonic time based on a arbitrary time to 
	//compute current time most accurately, m= represents mono time reading
}