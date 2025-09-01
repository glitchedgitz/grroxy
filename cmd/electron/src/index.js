// This file is the entry point for the Electron application.

const { app, BrowserWindow, ipcMain, nativeImage } = require('electron')
const path = require('path')

function createWindow() {
    const win = new BrowserWindow({
        width: 1080,
        height: 720,
        fullscreen: true,

        icon: __dirname + "/icons/grroxy.png",

        /* ------------- title-bar flags ------------- */
        titleBarStyle: 'hiddenInset',        // same “inset” look Wails uses
        transparent: true,
        title: 'Grroxy',

        /* ------------- transparent overlay -------- */
        titleBarOverlay: {                     // this draws the bar that slides in
            color: '#00000000',                // fully transparent (ARGB = 0×00)
            symbolColor: '#FFFFFF',            // traffic-light glyph colour
        },

        vibrancy: 'under-window',    // optional acrylic behind the whole win

        webPreferences: {
            preload: path.join(__dirname, 'preload.ts'),
            contextIsolation: true,
            nodeIntegration: false,
        }
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


    // Send fullscreen change to renderer
    // const sendFullscreenState = () => {
    //     win.webContents.send('fullscreen-changed', win.isFullScreen());
    // };

    // win.on('enter-full-screen', sendFullscreenState);
    // win.on('leave-full-screen', sendFullscreenState);
    win.on('enter-full-screen', () => {
        console.log('[main] Entered fullscreen');
        win.webContents.send('fullscreen-changed', true);
    });

    win.on('leave-full-screen', () => {
        console.log('[main] Left fullscreen');
        win.webContents.send('fullscreen-changed', false);
    });

    // Handler for isFullscreen
    ipcMain.handle('check-fullscreen', (event) => {
        const isFs = win.isFullScreen();
        console.log('[main] check-fullscreen →', isFs);
        return isFs;
    });

    app.dock.setIcon(nativeImage.createFromPath(__dirname + "/icons/grroxy.png"))


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



