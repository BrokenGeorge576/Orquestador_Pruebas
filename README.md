# Agregar un Nuevo Servicio al Orquestador de pruebas

Esta guía contiene los pasos necesarios para integrar y probar un nuevo servicio gRPC en el orquestador. 

## 1. Agregar los archivos gRPC (`pb/`)
Ya **no es necesario** subir los archivos `.proto`. 
Simplemente pega tus archivos `.pb.go` y `_grpc.pb.go` directamente dentro de la carpeta `pb/` del repositorio.

## 2. Configurar el archivo de prueba
Abre el archivo `cmd/orquestador/main.go` y busca la variable `archivo`. Modifícala para que apunte a la ruta local del archivo (pedimento) con el que deseas hacer la prueba:

```go
// cmd/orquestador/main.go
archivo := "/ruta/a/tu/archivo/de/prueba/m1617659.264"
```

```go
modulos := []ModuleConfig{
    // Servicio existente
    {
        Name:       "Fracciones",
        Host:       "localhost:50053",
        MethodName: "/apigrpc.FraccionesService/Fracciones",
    },
    // Aqui agregas tu servicio
    {
        Name:       "NombreDeTuServicio",
        Host:       "localhost:5005X",                           // Puerto donde corre tu servicio local
        MethodName: "/apigrpc.TuServicioService/NombreDelMetodo", // Ruta exacta del método gRPC
    },
}
```

El MethodName lo puedes encontrar en el archivo con terminacion `_grpc.pb.go` en la variable que termina en `FullMethodName`

Para correr el orquestador de pruebas asegurate de que tu servicio este arriba y procede a ejecutar el siguiente comando 
```cmd
go run cmd/orquestador/main.go
```
