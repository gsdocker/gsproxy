name "github.com/gsdocker/gsproxy"

plugin "github.com/gsmake/golang"
plugin "github.com/gsmake/gsrpc"


golang = {
    dependencies = {
        { name = "github.com/gsrpc/gorpc" };
    };

    tests = { "." }
}

gsrpc = {
    dependencies = {
        { name = "github.com/gsrpc/gorpc" };
    };
}
