extern char *rrdCreate(const char *filename, unsigned long step, time_t start, int argc, const char **argv);
extern char *rrdUpdate(const char *filename, const char *template, int argc, const char **argv);
extern char *rrdFetch(int *ret, char *filename, const char *cf, time_t *start, time_t *end, unsigned long *step, unsigned long *ds_cnt, char ***ds_namv, double **data);
extern char *rrdXport(int *ret, int argc, char **argv, int *xsize, time_t *start, time_t *end, unsigned long *step, unsigned long *col_cnt, char ***legend_v, double **data);
extern char *arrayGetCString(char **values, int i);
