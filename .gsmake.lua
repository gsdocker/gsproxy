name "github.com/gsdocker/gsproxy"

plugin "github.com/gsmake/golang"
plugin "github.com/gsmake/gsrpc"


properties.golang = {
    dependencies = {
        { name = "github.com/gsrpc/gorpc" };
    };

    tests = { "." }
}

properties.gsrpc = {
    dependencies = {
        { name = "github.com/gsrpc/gorpc" };
    };
}
