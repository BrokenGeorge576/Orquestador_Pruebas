# Orquestador para pruebas 

Este codigo es un orquestador construido en Go diseñado para procesar archivos de forma concurrente utilizando microservicios gRPC.
Su arquitectura está basada en datos (data-driven), lo que permite escalar y agregar nuevos módulos de procesamiento sin necesidad de 
modificar la lógica central del cliente gRPC.

## Requisitos Previos

Para ejecutar este código en cualquier máquina, necesitas tener instalado:

1. **Go** (Versión 1.24.0 o superior, según el `go.mod`).
2. **Protocol Buffers Compiler (`protoc`)**: Necesario para compilar los archivos `.proto`.
3. **Plugins de gRPC para Go**:
   Ejecuta los siguientes comandos para instalarlos si no los tienes:
   ```bash
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   ```
   *(Asegúrate de que la ruta de tu `$GOPATH/bin` o tu `GOPATH` esté en las variables de entorno de tu sistema).*

## Instalación y Ejecución Inicial

1. Clona o descarga este repositorio (rama optimizado).
2. Abre una terminal en la raíz del proyecto.
3. Descarga las dependencias ejecutando:
   ```bash
   go mod tidy
   ```
4. Para correr el orquestador:
   ```bash
   go run cmd/orquestador/main.go
   ```

---

## Gestión de Archivos Protobuf (`.proto`)

Todos los contratos de los microservicios deben colocarse en la carpeta `proto/`. 

### Reglas estrictas para los archivos `.proto`:
Para mantener la estructura limpia y evitar errores de dependencias cíclicas o rutas rotas, **TODOS** los archivos `.proto` que 
agregues deben incluir obligatoriamente esta línea en su cabecera, modifica tu proto para que quede de esta manera en esta linea:

```proto
option go_package = "./pb";
```

Esto le indica al compilador que los archivos generados en Go (`.pb.go` y `_grpc.pb.go`) deben guardarse exclusivamente en la carpeta `pb/` del proyecto. 

### Cómo compilar un nuevo servicio Protobuf:
Cuando agregues o modifiques un archivo `.proto` en la carpeta `proto/`, abre tu terminal en la **raíz del proyecto** y ejecuta el siguiente comando:

```bash
protoc --go_out=. --go-grpc_out=. proto/nombre_del_archivo.proto
```
*Si quieres compilar todos de golpe, puedes usar `protoc --go_out=. --go-grpc_out=. proto/*.proto`.*

---

## ⚙️ Cómo Agregar o Quitar Servicios al Orquestador

Una de las mayores ventajas de este orquestador para pruebas es que **no necesitas modificar el cliente gRPC (`internal/grpcclient/client.go`) para agregar nuevos microservicios**. 

Toda la configuración se maneja en el archivo `cmd/orquestador/main.go`.

### Flujo Base 
El primer servicio que se ejecuta es `ProcessRestSintactica`. Este servicio es crítico y extrae el "Documento" base. Su ejecución está separada y ocurre antes del procesamiento en paralelo.

### Agregar nuevos módulos
Para conectar un nuevo servicio, solo debes agregar una nueva entrada al arreglo `modulos` dentro de `main.go`. 

Utiliza la estructura `ModuleConfig`:

```go
modulos := []ModuleConfig{
    {
        Name:       "NombreParaConsola",        // Identificador (ej. "Fracciones")
        Host:       "IP:PUERTO",                // Dónde está desplegado (ej. "192.168.0.11:50051")
        MethodName: "/Paquete.Servicio/Metodo", // Ruta gRPC del método a ejecutar
    },
    // ... agrega más servicios aquí
}
```

### ¿De dónde saco el `MethodName`?
Una vez que compiles tu `.proto`, abre el archivo generado `pb/tu_archivo_grpc.pb.go`. 
Busca la constante que termina en `_FullMethodName` y copia el string exacto que contiene (Ejemplo: `"/apigrpc.SIMPLIFICADOS/SIMPLIFICADOS"`).

### Tolerancia a Fallos
Si eliminas un módulo del arreglo `modulos`, el orquestador simplemente dejará de llamarlo. 
Si agregas un módulo pero el servicio no está levantado (o falla la conexión), **el orquestador no se detendrá**. Lanzará una advertencia en 
consola indicando que el servicio no es alcanzable e incluirá un registro del error en el JSON final, permitiendo que el resto de los módulos 
terminen su trabajo sin interrupciones.
