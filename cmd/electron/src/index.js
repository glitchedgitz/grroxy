// This file is the entry point for the Electron application.

const { app, BrowserWindow, ipcMain, nativeImage } = require('electron')
const path = require('path')

let mainWindow = null

function createWindow() {
    const iconPath = path.resolve(__dirname, "icons", "grroxy.png")

    mainWindow = new BrowserWindow({
        width: 1080,
        height: 720,
        fullscreen: true,

        icon: iconPath,

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
        mainWindow.loadFile(`${__dirname}/frontend/dist/index.html`)
    } else {
        // Load vite dev server page 
        console.log('Development mode')
        mainWindow.loadURL('http://localhost:5173/')
        // mainWindow.loadFile(`${__dirname}/frontend/dist/index.html`)

    }

    // setTimeout(() => {
    //     mainWindow.webContents.openDevTools()
    // }, 5000)


    // Send fullscreen change to renderer
    // const sendFullscreenState = () => {
    //     mainWindow.webContents.send('fullscreen-changed', mainWindow.isFullScreen());
    // };

    // mainWindow.on('enter-full-screen', sendFullscreenState);
    // mainWindow.on('leave-full-screen', sendFullscreenState);
    mainWindow.on('enter-full-screen', () => {
        console.log('[main] Entered fullscreen');
        mainWindow.webContents.send('fullscreen-changed', true);
    });

    mainWindow.on('leave-full-screen', () => {
        console.log('[main] Left fullscreen');
        mainWindow.webContents.send('fullscreen-changed', false);
    });

    app.dock.setIcon(nativeImage.createFromPath(iconPath))


}

app.whenReady()
    .then(() => {
        // Register IPC handlers once when app is ready
        ipcMain.handle('check-fullscreen', (event) => {
            if (mainWindow) {
                const isFs = mainWindow.isFullScreen();
                console.log('[main] check-fullscreen →', isFs);
                return isFs;
            }
            return false;
        });

        createWindow()

        app.on('activate', function () {
            if (BrowserWindow.getAllWindows().length === 0) createWindow()
        })




    })

app.on('window-all-closed', function () {
    if (process.platform !== 'darwin') app.quit()
})



