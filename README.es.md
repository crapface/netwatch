# NetWatch — Escáner y Monitor de Red Portátil

NetWatch descubre equipos en su red que tienen ciertos puertos TCP abiertos, luego
los vigila y le envía un correo cuando alguno deja de responder. Es un único
ejecutable de Windows portátil: sin instalador, sin runtime, sin permisos de
administrador, sin registro, sin `%APPDATA%`. Todo lo que escribe (registro, caché
de fabricantes, perfiles, informes) queda en la misma carpeta que el `.exe`.

[English](README.md) | Español

---

## 1. Requisitos

* Windows 10 u 11 (64 bits). También funciona en Windows 7/8.1.
* Nada más. NetWatch es un binario nativo autónomo.

## 2. Cómo ejecutarlo

1. Copie `NetWatch.exe` a donde quiera: una memoria USB, `C:\Tools\NetWatch\`, el escritorio.
2. **Haga doble clic en `NetWatch.exe`.** Eso es todo.

En el primer inicio, Windows SmartScreen puede advertir que el editor es desconocido
(el binario no está firmado). Haga clic en **Más información → Ejecutar de todas formas**.

## 3. Configuración inicial

1. **Rango de red.** En la pestaña **Escáner** se detecta y completa automáticamente
   la subred activa (p. ej. `192.168.1.0/24`). Puede editarla libremente: escriba
   cualquier CIDR como `10.0.0.0/16` o `172.16.5.0/24`. Pulse **Detectar** para volver a detectar.
2. **Puertos.** Vaya a **Ajustes → Puertos a escanear**. Los predeterminados son
   `8080` y `3000`. Agregue o quite los que desee (escriba un número y pulse
   **Agregar**; seleccione uno y pulse **Quitar seleccionado**).
3. **Nombres de fabricante (opcional).** Ajustes → **Base de fabricantes (IEEE OUI)**
   → **Actualizar datos OUI**. NetWatch descarga la lista oficial OUI de IEEE y la
   guarda como `oui_cache.json` junto al exe, para mostrar el fabricante de cada MAC.
4. **Correo (opcional).** Ajustes → **Notificaciones por correo (SMTP)**. Indique
   servidor, puerto, cifrado (Ninguno / STARTTLS / SSL), usuario, contraseña,
   remitente y destinatario. Marque **Activar alertas por correo** y pulse
   **Probar correo** para confirmar. ⚠ La contraseña SMTP se guarda en **texto plano**
   dentro del perfil `.site`; NetWatch se lo advierte en la interfaz.
5. **Idioma.** Use el menú desplegable arriba a la derecha para cambiar entre English
   y Español en cualquier momento. Toda la interfaz se actualiza al instante.

## 4. Flujo de trabajo: Escanear → Monitorear → Guardar → Informe

**Escanear.** En la pestaña Escáner, confirme el rango y pulse **Escanear**. Una barra
de progreso avanza mientras NetWatch prueba cada puerto en cada dirección de la
subred. No hay límite de tiempo —está diseñado para ser exhaustivo— pero puede pulsar
**Cancelar** cuando quiera y conservar lo encontrado hasta ese momento. La interfaz
nunca se congela. Los equipos descubiertos aparecen con estado, IP, nombre,
fabricante, MAC, puertos abiertos e identificador único.

**Monitorear.** Al terminar el escaneo aparece un gran botón **intermitente**
*INICIAR MONITOREO* bajo la tabla. Púlselo. NetWatch revisa solo los equipos recién
encontrados, cada *N* segundos (60 por defecto, configurable en Ajustes). La pestaña
**Monitor** muestra un indicador en vivo 🟢 ACTIVO / 🔴 CAÍDO por equipo y un registro
de eventos. Un equipo se marca **CAÍDO** solo tras fallar en **todos** sus puertos
abiertos durante **dos comprobaciones consecutivas** (esto evita falsas alarmas).
Cuando ocurre, recibe un correo; no se le vuelve a avisar hasta que el equipo se
recupere y caiga de nuevo.

**Guardar el sitio.** Ajustes → ponga un nombre al sitio → **Guardar sitio**. Esto
escribe un único archivo `.site` con el rango de red, la lista de puertos, la
configuración de correo, la lista completa de equipos, todo el registro de eventos de
monitoreo y si el monitoreo estaba activo.

**Generar un informe.** Ajustes → **Generar informe** crea un
`Report_<Sitio>_<fecha>.html` autónomo (todo el CSS incrustado: un solo archivo para
enviar o archivar) y lo abre en el navegador. Lista cada equipo y el registro completo
de eventos ACTIVO/CAÍDO.

## 5. Cargar un sitio guardado y reanudar

Ajustes → **Cargar sitio** → elija su archivo `.site`. NetWatch restaura la lista de
equipos, el registro de eventos y todos los ajustes tal como se guardaron. Si el sitio
estaba monitoreando al guardarse, NetWatch ofrece **reanudar el monitoreo** de
inmediato; así puede cerrar la app, volver a abrirla y continuar donde lo dejó.

## 6. Referencia de ajustes

| Ajuste | Significado |
|---|---|
| Puertos a escanear | Puertos probados; un equipo se "encuentra" si ≥1 está abierto. |
| Concurrencia máx. | Conexiones TCP simultáneas (100 por defecto). |
| Tiempo de espera (ms) | Espera por cada conexión (1000 por defecto). |
| Reintentos por timeout | Intentos extra ante timeout (1 por defecto). |
| Intervalo de comprobación | Segundos entre pasadas de monitoreo (60 por defecto). |
| Correo | Servidor/puerto/cifrado/credenciales/destinatario + Probar correo. |
| Actualizar datos OUI | Descargar y cachear la base de fabricantes IEEE. |
| Guardar / Cargar sitio | Conservar o restaurar todo el estado (`.site`). |
| Generar informe | Escribir y abrir el informe HTML autónomo. |

## 7. Archivos que crea NetWatch (portátiles, junto al .exe)

* `app.log` — registro con marca de tiempo INFO/WARN/ERROR.
* `oui_cache.json` — base de fabricantes IEEE cacheada (tras Actualizar datos OUI).
* `profiles\` — carpeta predeterminada de los diálogos Guardar/Cargar.
* `Report_*.html` — informes generados.

Puede borrar cualquiera de ellos; NetWatch recrea lo que necesite.

## 8. Solución de problemas

* **El escaneo no encuentra nada.** Confirme que el CIDR coincide con su LAN (revise
  `ipconfig`). Asegúrese de escanear puertos que algo realmente sirva. Las redes
  corporativas suelen bloquear el tráfico entre equipos ("aislamiento de cliente").
* **Las columnas Fabricante / MAC están vacías.** Las MAC provienen de la caché ARP
  del sistema operativo, que solo conoce equipos de su **propio** segmento de capa 2.
  Para subredes enrutadas o remotas la MAC (y el fabricante) no están disponibles:
  es lo esperado y se maneja con elegancia. Además, ejecute **Actualizar datos OUI**.
* **Falla la prueba de correo.** Revise servidor, puerto y cifrado. Puerto 587 →
  STARTTLS, 465 → SSL/TLS, 25 → Ninguno. Para proveedores que exigen contraseñas de
  aplicación (Gmail/Microsoft 365), use una contraseña de aplicación. Como último
  recurso para servidores con certificado autofirmado, marque *Omitir verificación
  del certificado TLS*.
* **Firewall de Windows.** Las conexiones TCP salientes (escaneo) y SMTP suelen estar
  permitidas sin avisos. NetWatch no necesita reglas de entrada.
* **Subredes grandes.** Un `/16` (65 000 direcciones) funciona pero tarda; el escáner
  transmite direcciones en flujo para no agotar la memoria. Rangos mayores a `/8` se rechazan.

## 9. Compilar desde el código fuente

Requiere **Go 1.22+** (<https://go.dev/dl/>). Nada más: la compilación es Go puro
(`CGO_ENABLED=0`) y el manifiesto + icono de Windows ya están incrustados en
`rsrc_windows_amd64.syso`, por lo que un simple `go build` produce un ejecutable con
temas visuales, compatible con DPI y sin elevación.

```powershell
.\build.ps1            # o: build.bat
```

El paquete portátil queda en `.\dist`. Ejecute las pruebas con `go test ./internal/...`.

## 10. Justificación de la tecnología

**Go + [lxn/walk](https://github.com/lxn/walk).** Los requisitos duros son: un único
ejecutable de Windows que se abra con doble clic, **sin** instalación de runtime; una
GUI nativa con pestañas (campos editables, una tabla con estado en vivo por fila, un
botón intermitente); E/S de red concurrente intensa que nunca bloquee la interfaz y
sea cancelable; y portabilidad total (sin registro ni `%APPDATA%`).

Go compila a un único `.exe` estático y sin dependencias (~10 MB), y sus goroutines +
cancelación por `context` encajan perfectamente con un escáner de subred de
concurrencia acotada. `lxn/walk` envuelve los controles nativos de Win32 (TabWidget,
TableView, ProgressBar), por lo que la app se ve nativa, es ligera y —clave— está
**libre de CGO**: sin compilador de C y con compilación cruzada limpia. Las
operaciones largas corren en goroutines y devuelven las actualizaciones de UI con
`Synchronize` de `walk`, manteniendo la ventana fluida y cancelable. El botón
intermitente es un `walk.Composite` cuyo fondo alterna mediante un temporizador de
500 ms; el estado en vivo es un estilizador de celdas que colorea cada fila de verde o
rojo. La localización es JSON incrustado que se aplica reetiquetando cada control al
vuelo, de modo que el idioma cambia al instante sin reiniciar.

---

NetWatch v1.0.0 · vea [CHANGELOG.md](CHANGELOG.md).
