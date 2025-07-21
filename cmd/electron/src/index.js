// This file is the entry point for the Electron application.

const { app, BrowserWindow } = require('electron')

function createWindow() {
    const win = new BrowserWindow({
        width: 1080,
        height: 720,
        fullscreen: true,

        /* ------------- title-bar flags ------------- */
        titleBarStyle: 'hiddenInset',        // same “inset” look Wails uses
        transparent: true,
        title: 'Grroxy',

        /* ------------- transparent overlay -------- */
        titleBarOverlay: {                   // this draws the bar that slides in
            color: '#00000000',                // fully transparent (ARGB = 0×00)
            symbolColor: '#FFFFFF',            // traffic-light glyph colour
        },

        vibrancy: 'under-window'    // optional acrylic behind the whole win
    })

    // win.setWindowButtonVisibility(false)

    if (process.env.NODE_ENV !== 'development') {
        // Load production build
        win.loadFile(`${__dirname}/frontend/dist/index.html`)
    } else {
        // Load vite dev server page 
        console.log('Development mode')
        win.loadURL('http://localhost:5173/')
        // win.loadFile(`${__dirname}/frontend/dist/index.html`)

    }

    // setTimeout(() => {
    //     win.webContents.openDevTools()
    // }, 5000)
}

app.whenReady()
    .then(() => {
        createWindow()

        app.on('activate', function () {
            if (BrowserWindow.getAllWindows().length === 0) createWindow()
        })
    })

app.on('window-all-closed', function () {
    if (process.platform !== 'darwin') app.quit()
})
