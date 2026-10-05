// Minimal Qt6 Wayland test client for the hub-stability research (a stand-in viewer).
// Shows a counter, prints one line per second so a test can see it kept running.
#include <QApplication>
#include <QWidget>
#include <QPainter>
#include <QTimer>
#include <QClipboard>
#include <QDateTime>
#include <cstdio>

class W : public QWidget {
public:
    int n = 0;
    explicit W(const QString &name) : name_(name) {
        setWindowTitle(name);
        resize(480, 270);
        auto *t = new QTimer(this);
        connect(t, &QTimer::timeout, this, [this] {
            n++;
            std::printf("%s tick %d pid-alive t=%lld screens=%d\n", qPrintable(name_), n,
                        (long long)QDateTime::currentMSecsSinceEpoch(),
                        (int)QGuiApplication::screens().size());
            std::fflush(stdout);
            update();
        });
        t->start(1000);
    }
protected:
    void paintEvent(QPaintEvent *) override {
        QPainter p(this);
        p.fillRect(rect(), QColor(40, 90, 160));
        p.setPen(Qt::white);
        p.drawText(rect(), Qt::AlignCenter, QString("%1\ntick %2").arg(name_).arg(n));
    }
private:
    QString name_;
};

int main(int argc, char **argv) {
    QApplication app(argc, argv);
    QString name = argc > 1 ? argv[1] : "qtc";
    QGuiApplication::setDesktopFileName("hubos-" + name);  // becomes the Wayland app_id
    W w(name);
    w.show();
    return app.exec();
}
